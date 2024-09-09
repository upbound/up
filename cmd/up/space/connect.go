// Copyright 2024 Upbound Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package space

import (
	"context"
	"fmt"
	"path"
	"slices"
	"strconv"
	"strings"

	"github.com/alecthomas/kong"
	"github.com/google/uuid"
	"github.com/pterm/pterm"
	"helm.sh/helm/v3/pkg/storage/driver"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane/crossplane-runtime/pkg/errors"

	upboundv1alpha1 "github.com/upbound/up-sdk-go/apis/upbound/v1alpha1"
	sdkerrs "github.com/upbound/up-sdk-go/errors"
	"github.com/upbound/up-sdk-go/service/accounts"
	"github.com/upbound/up-sdk-go/service/organizations"
	"github.com/upbound/up-sdk-go/service/robots"
	"github.com/upbound/up-sdk-go/service/tokens"
	"github.com/upbound/up/internal/install/helm"
	"github.com/upbound/up/internal/undo"
	"github.com/upbound/up/internal/upbound"
	"github.com/upbound/up/internal/upterm"
	"github.com/upbound/up/internal/version"
)

const (
	agentChart   = "agent"
	agentNs      = "upbound-system"
	agentSecret  = "space-token"
	connConfMap  = "space-connect"
	mxpConfigMap = "mxp-config"

	keySpace   = "space"
	keyToken   = "token"
	keyRobotID = "robotID"
	keyTokenID = "tokenID"
)

const (
	// TODO(tnthornton) maybe move this to the agent chart?
	devConnect  = "tls://connect.u5d.dev"
	stagConnect = "tls://connect.staging-eikeagoo.upbound.services"
	prodConnect = "tls://connect.upbound.io"
)

type connectCmd struct {
	Registry authorizedRegistryFlags `embed:""`

	Upbound upbound.Flags     `embed:""`
	Kube    upbound.KubeFlags `embed:""`

	Space string `arg:"" optional:"" help:"Name of the Upbound Space. If name is not a supplied, one is generated."`
	Token string `name:"robot-token" optional:"" hidden:"" help:"The Upbound robot token contents used to authenticate the connection."`

	Environment string `name:"up-environment" env:"UP_ENVIRONMENT" default:"prod" hidden:"" help:"Override the default Upbound Environment."`
}

func (c *connectCmd) AfterApply(kongCtx *kong.Context) error {
	upCtx, err := upbound.NewFromFlags(c.Upbound)
	if err != nil {
		return err
	}
	upCtx.SetupLogging()

	cfg, err := upCtx.BuildSDKConfig()
	if err != nil {
		return err
	}
	ctrlCfg, err := upCtx.BuildControllerClientConfig()
	if err != nil {
		return err
	}

	kongCtx.Bind(upCtx)
	kongCtx.Bind(ctrlCfg)
	kongCtx.Bind(accounts.NewClient(cfg))
	kongCtx.Bind(organizations.NewClient(cfg))
	kongCtx.Bind(robots.NewClient(cfg))
	kongCtx.Bind(tokens.NewClient(cfg))

	if err := c.Kube.AfterApply(); err != nil {
		return err
	}

	// NOTE(tnthornton) we currently only have support for stylized output.
	pterm.EnableStyling()
	upterm.DefaultObjPrinter.Pretty = true

	kubeconfig := c.Kube.GetConfig()

	kClient, err := kubernetes.NewForConfig(kubeconfig)
	if err != nil {
		return err
	}
	kongCtx.Bind(kClient)

	mgr, err := helm.NewManager(kubeconfig,
		agentChart,
		c.Registry.Repository,
		helm.WithNamespace(agentNs),
		helm.WithBasicAuth(c.Registry.Username, c.Registry.Password),
		helm.IsOCI(),
		helm.Wait(),
		helm.Force(true),
		helm.RollbackOnError(true),
	)
	if err != nil {
		return err
	}
	kongCtx.Bind(mgr)

	return nil
}

// Run executes the connect command.
func (c *connectCmd) Run(ctx context.Context, mgr *helm.Installer, kClient *kubernetes.Clientset, upCtx *upbound.Context, ac *accounts.Client, oc *organizations.Client, tc *tokens.Client, rc *robots.Client, rest *rest.Config) (rErr error) { //nolint:gocyclo
	mxpConfig, err := kClient.CoreV1().ConfigMaps(agentNs).Get(ctx, mxpConfigMap, metav1.GetOptions{})
	if kerrors.IsNotFound(err) {
		return errors.New("failed to detect Space. Please run `up space init` first.")
	} else if err != nil {
		return errors.Wrapf(err, `failed to get ConfigMap "%s/%s"`, agentNs, mxpConfigMap)
	}
	if spaceAcc := mxpConfig.Data["account"]; spaceAcc != upCtx.Account {
		return errors.Errorf("account of the Space %q and account of the profile %q mismatch. Use `--account=%s` to connect to the right organization.", spaceAcc, c.Upbound.Account, spaceAcc)
	}

	connectSpinner, err := upterm.CheckmarkSuccessSpinner.Start("Connecting Space to Upbound Console...")
	if err != nil {
		return err
	}
	defer func() {
		if rErr != nil {
			connectSpinner.Fail(rErr)
		}
	}()
	return undo.Do(func(u undo.Undoer) error {
		sc, err := client.New(rest, client.Options{})
		if err != nil {
			return err
		}

		a, err := upbound.GetOrganization(ctx, ac, upCtx.Account)
		if err != nil {
			return err
		}

		cc, err := createConnectConfigmap(ctx, connectSpinner.InfoPrinter, kClient, agentNs, connConfMap, u)
		if err != nil {
			return err
		}

		if err := c.prepareSpace(ctx, connectSpinner, kClient, a, ac, oc, rc, sc, u, &cc); err != nil {
			return err
		}
		connectSpinner.UpdateText(fmt.Sprintf("Connecting Space %q to Upbound Console...", cc.Data[keySpace]))

		if err := c.prepareToken(ctx, connectSpinner, kClient, a, rc, oc, tc, u, &cc); err != nil {
			return err
		}

		connectSpinner.UpdateText("Installing Upbound agent...")
		if err := c.createNamespace(ctx, connectSpinner.InfoPrinter, kClient, agentNs, u); err != nil {
			return err
		}
		if err := c.createTokenSecret(ctx, connectSpinner.InfoPrinter, kClient, agentNs, agentSecret, u); err != nil {
			return err
		}
		if err := c.installAgent(connectSpinner.InfoPrinter, mgr, a, u); err != nil {
			return err
		}
		connectSpinner.Success(fmt.Sprintf("Space %q is connected to Upbound Console", c.Space))
		return nil
	})
}

func (c *connectCmd) installAgent(p pterm.TextPrinter, mgr *helm.Installer, a *accounts.AccountResponse, u undo.Undoer) error {
	v, err := mgr.GetCurrentVersion()
	if err == nil {
		return c.upgradeAgent(p, mgr, a, v, u)
	}
	if !errors.Is(err, driver.ErrReleaseNotFound) {
		return errors.Wrapf(err, `failed to lookup Chart "%s/%s"`, agentNs, agentChart)
	}

	p.Printfln(`Installing Chart "%s/%s"`, agentNs, agentChart)
	if err := mgr.Install(version.AgentVersion(), c.deriveParams(a)); err != nil {
		return errors.Wrapf(err, `failed to install Chart "%s/%s"`, agentNs, agentChart)
	}
	u.Undo(func() error {
		if err := mgr.Uninstall(); err != nil {
			return errors.Wrapf(err, `failed to uninstall Chart "%s/%s"`, agentNs, agentChart)
		}
		p.Printfln(`Chart "%s/%s" uninstalled`, agentNs, agentChart)
		return nil
	})
	p.Printfln(`Chart "%s/%s" version %s installed`, agentNs, agentChart, version.AgentVersion())
	return nil
}

func (c *connectCmd) upgradeAgent(p pterm.TextPrinter, mgr *helm.Installer, a *accounts.AccountResponse, currentVersion string, u undo.Undoer) error {
	if currentVersion != version.AgentVersion() {
		p.Printfln(`Upgrading Chart "%s/%s" %s => %s`, agentNs, agentChart, currentVersion, version.AgentVersion())
	} else {
		p.Printfln(`Reinstalling Chart "%s/%s" %s`, agentNs, agentChart, version.AgentVersion())
	}
	if err := mgr.Upgrade(version.AgentVersion(), c.deriveParams(a)); err != nil {
		return errors.Wrapf(err, `failed to upgrade Chart "%s/%s"`, agentNs, agentChart)
	}
	u.Undo(func() error {
		if err := mgr.Rollback(); err != nil {
			return errors.Wrapf(err, `failed to rollback Chart "%s/%s"`, agentNs, agentChart)
		}
		p.Printfln(`Chart "%s/%s" rolled back`, agentNs, agentChart)
		return nil
	})
	return nil
}

func (c *connectCmd) deriveParams(a *accounts.AccountResponse) map[string]any {
	connectURL := prodConnect
	switch c.Environment {
	case "dev":
		connectURL = devConnect
	case "staging":
		connectURL = stagConnect
	}

	params := map[string]any{
		"connect": map[string]any{
			"url": connectURL,
		},
		"space":        c.Space,
		"organization": a.Organization.Name,
		"tokenSecret":  agentSecret,
		"image": map[string]any{
			"repository": c.Registry.Repository.JoinPath("agent").String(),
		},
		"registration": map[string]any{
			"image": map[string]any{
				"repository": c.Registry.Repository.JoinPath("register-init").String(),
			},
		},
		"imagePullSecrets": []map[string]string{
			{
				"name": defaultImagePullSecret,
			},
		},
	}

	if c.Environment != "prod" {
		params["extraArgs"] = []string{
			fmt.Sprintf("--env=%s", c.Environment),
		}
	}
	return params
}

func (c *connectCmd) prepareToken(ctx context.Context, connectSpinner *pterm.SpinnerPrinter, kClient *kubernetes.Clientset, a *accounts.AccountResponse, rc *robots.Client, oc *organizations.Client, tc *tokens.Client, u undo.Undoer, cmr **corev1.ConfigMap) error {
	if c.Token == "" {
		connectSpinner.UpdateText("Generating agent Robot and Token...")

		rid, err := c.createRobot(ctx, connectSpinner.InfoPrinter, kClient, a, rc, oc, u, cmr)
		if err != nil {
			return err
		}

		res, err := c.createToken(ctx, connectSpinner, kClient, a, rc, tc, rid, u, cmr)
		if err != nil {
			return err
		}
		c.Token = fmt.Sprint(res.Meta["jwt"])
	}
	return nil
}

func (c *connectCmd) prepareSpace(ctx context.Context, connectSpinner *pterm.SpinnerPrinter, kClient *kubernetes.Clientset, a *accounts.AccountResponse, ac *accounts.Client, oc *organizations.Client, rc *robots.Client, sc client.Client, u undo.Undoer, cmr **corev1.ConfigMap) error { //nolint:gocyclo
	cm := *cmr
	space := &upboundv1alpha1.Space{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: a.Organization.Name,
			Name:      c.Space,
		},
		Spec: upboundv1alpha1.SpaceSpec{},
	}
	// auto generate space name if none given.
	if space.Name == "" {
		space.GenerateName = "connected-"
	}
	if v, ok := cm.Data[keySpace]; ok {
		parts := strings.Split(v, "/")
		if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
			return fmt.Errorf("invalid space name %q", v)
		}
		ns, name := parts[0], parts[1]
		if (space.Name != "" && space.Name != name) || space.Namespace != ns {
			if err := warnAndConfirmWithSpinner(connectSpinner,
				`Space "%s/%s" is currently connected to Upbound Console. Would you like to continue?`+"\n\n"+
					`  By continuing the current Space will be removed and this Space will be connected as "%s/%s" instead.`+"\n",
				ns, name, space.Namespace, space.Name,
			); err != nil {
				return err
			}
			if err := disconnectSpace(ctx, connectSpinner, ac, oc, rc, sc, ns, name); err != nil {
				return err
			}
			delete(cm.Data, keySpace)
			var err error
			cm, err = kClient.CoreV1().ConfigMaps(agentNs).Update(ctx, cm, metav1.UpdateOptions{})
			if err != nil {
				return errors.Wrapf(err, `failed to update ConfigMap "%s/%s"`, agentNs, connConfMap)
			}
			*cmr = cm
		}
	}
	name, err := c.createSpace(ctx, connectSpinner, kClient, space, sc, u, cmr)
	if err != nil {
		return err
	}
	c.Space = name
	return nil
}

func (c *connectCmd) createSpace(ctx context.Context, connectSpinner *pterm.SpinnerPrinter, kClient *kubernetes.Clientset, space *upboundv1alpha1.Space, sc client.Client, u undo.Undoer, cmr **corev1.ConfigMap) (string, error) {
	cm := *cmr
	if space.Name == "" {
		connectSpinner.UpdateText(fmt.Sprintf("Creating a new Space in Upbound Console in %q...", space.Namespace))
	} else {
		connectSpinner.UpdateText(fmt.Sprintf(`Creating Space "%s/%s" in Upbound Console...`, space.Namespace, space.Name))
	}

	connectSpinner.InfoPrinter.Printfln(`Creating Space "%s/%s"`, space.Namespace, space.Name)
	err := sc.Create(ctx, space)
	if err == nil {
		u.Undo(func() error {
			return deleteSpace(ctx, connectSpinner.InfoPrinter, sc, space.Namespace, c.Space)
		})

		connectSpinner.InfoPrinter.Printfln(`Space "%s/%s" created`, space.Namespace, space.Name)
		cm.Data[keySpace] = path.Join(space.Namespace, space.Name)
		cm, err = kClient.CoreV1().ConfigMaps(agentNs).Update(ctx, cm, metav1.UpdateOptions{})
		if err != nil {
			return "", errors.Wrapf(err, `failed to update ConfigMap "%s/%s"`, agentNs, connConfMap)
		}
		*cmr = cm
		return space.Name, nil
	}
	if !kerrors.IsAlreadyExists(err) {
		return "", errors.Wrapf(err, errCreateSpace)
	}
	connectSpinner.InfoPrinter.Printfln(`Space "%s/%s" exists`, space.Namespace, space.Name)
	if err := warnAndConfirmWithSpinner(connectSpinner,
		`Space "%s/%s" already exists. Would you like to overwrite it?`+"\n\n"+
			"  If the other Space cluster still exists, the Upbound agent will be left running and you will need to delete it manually.\n",
		space.Namespace, space.Name,
	); err != nil {
		return "", err
	}
	connectSpinner.UpdateText(fmt.Sprintf(`Connecting Space "%s/%s" to Upbound Console...`, space.Namespace, space.Name))
	if cm.Data[keySpace] == path.Join(space.Namespace, c.Space) {
		return c.Space, nil
	}
	cm.Data[keySpace] = path.Join(space.Namespace, c.Space)
	cm, err = kClient.CoreV1().ConfigMaps(agentNs).Update(ctx, cm, metav1.UpdateOptions{})
	if err != nil {
		return "", errors.Wrapf(err, `failed to update ConfigMap "%s/%s"`, agentNs, connConfMap)
	}
	*cmr = cm
	return space.Name, nil
}

func (c *connectCmd) createNamespace(ctx context.Context, p pterm.TextPrinter, kClient *kubernetes.Clientset, ns string, u undo.Undoer) error {
	_, err := kClient.CoreV1().Namespaces().Create(ctx, &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: ns,
		},
	}, metav1.CreateOptions{})
	if kerrors.IsAlreadyExists(err) {
		return nil
	}
	if err != nil {
		return errors.Wrap(err, fmt.Sprintf(errFmtCreateNamespace, ns))
	}
	u.Undo(func() error {
		return c.deleteNamespace(ctx, p, kClient, agentNs)
	})
	return nil
}

func (c *connectCmd) deleteNamespace(ctx context.Context, p pterm.TextPrinter, kClient *kubernetes.Clientset, ns string) error {
	p.Printfln("Deleting Namespace %q")
	if err := kClient.CoreV1().Namespaces().Delete(ctx, ns, metav1.DeleteOptions{}); err != nil {
		return err
	}
	p.Printfln("Namespace %q deleted")
	return nil
}

func getConnectConfigmap(ctx context.Context, kClient *kubernetes.Clientset, ns, name string) (*corev1.ConfigMap, error) {
	return kClient.CoreV1().ConfigMaps(ns).Get(ctx, name, metav1.GetOptions{})
}
func createConnectConfigmap(ctx context.Context, p pterm.TextPrinter, kClient *kubernetes.Clientset, ns, name string, u undo.Undoer) (*corev1.ConfigMap, error) {
	cm, err := getConnectConfigmap(ctx, kClient, ns, name)
	if err == nil {
		if cm.Data == nil {
			cm.Data = map[string]string{}
		}
		p.Printfln(`ConfigMap "%s/%s" found, resuming...`, ns, name)
		return cm, nil
	}
	if !kerrors.IsNotFound(err) {
		return nil, errors.Wrapf(err, `failed to get ConfigMap "%s/%s"`, ns, name)
	}
	p.Printfln(`Creating ConfigMap "%s/%s" to track connect progress...`, ns, name)
	cm, err = kClient.CoreV1().ConfigMaps(ns).Create(ctx, &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Data: map[string]string{},
	}, metav1.CreateOptions{})
	if err != nil {
		return nil, errors.Wrapf(err, `failed to create ConfigMap "%s/%s"`, ns, name)
	}
	u.Undo(func() error {
		return deleteConnectConfigmap(ctx, p, kClient, ns, name)
	})
	if cm.Data == nil {
		cm.Data = map[string]string{}
	}
	p.Printfln(`ConfigMap "%s/%s" created`, ns, name)
	return cm, nil
}

func deleteConnectConfigmap(ctx context.Context, p pterm.TextPrinter, kClient *kubernetes.Clientset, ns, name string) error {
	p.Printfln(`Deleting ConfigMap "%s/%s".`, ns, name)
	if err := kClient.CoreV1().ConfigMaps(ns).Delete(ctx, name, metav1.DeleteOptions{}); err != nil && !kerrors.IsNotFound(err) {
		return errors.Wrapf(err, `failed to delete ConfigMap "%s/%s"`, ns, name)
	}
	p.Printfln(`ConfigMap "%s/%s" deleted.`, ns, name)
	return nil
}

func (c *connectCmd) createTokenSecret(ctx context.Context, p pterm.TextPrinter, kClient *kubernetes.Clientset, ns, name string, u undo.Undoer) error {
	p.Printfln(`Creating Secret "%s/%s"`, ns, name)
	_, err := kClient.CoreV1().Secrets(ns).Create(ctx, &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Data: map[string][]byte{
			keySpace: []byte(c.Space),
			keyToken: []byte(c.Token),
		},
	}, metav1.CreateOptions{})
	if err == nil {
		u.Undo(func() error {
			return deleteTokenSecret(ctx, p, kClient, agentNs, agentSecret)
		})
		p.Printfln(`Secret "%s/%s" created`, ns, name)
		return nil
	}
	if !kerrors.IsAlreadyExists(err) {
		return errors.Wrapf(err, `failed to create Secret "%s/%s"`, ns, name)
	}
	p.Printfln(`Secret "%s/%s" exists, updating...`, ns, name)
	// secret already exists, replace it
	s, err := kClient.CoreV1().Secrets(ns).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return errors.Wrapf(err, `failed to get Secret "%s/%s"`, ns, name)
	}
	if slices.Equal(s.Data[keySpace], []byte(c.Space)) && slices.Equal(s.Data[keyToken], []byte(c.Token)) {
		u.Undo(func() error {
			return deleteTokenSecret(ctx, p, kClient, agentNs, agentSecret)
		})
		return nil
	}
	if s.Data == nil {
		s.Data = map[string][]byte{}
	}
	s.Data[keySpace] = []byte(c.Space)
	s.Data[keyToken] = []byte(c.Token)
	_, err = kClient.CoreV1().Secrets(ns).Update(ctx, s, metav1.UpdateOptions{})
	if err != nil {
		return errors.Wrapf(err, `failed to update Secret "%s/%s"`, ns, name)
	}
	u.Undo(func() error {
		return deleteTokenSecret(ctx, p, kClient, agentNs, agentSecret)
	})
	p.Printfln(`Secret "%s/%s" updated`, ns, name)
	return nil
}

func deleteTokenSecret(ctx context.Context, p pterm.TextPrinter, kClient *kubernetes.Clientset, ns, name string) error {
	p.Printfln(`Deleting Secret "%s/%s"`, ns, name)
	if err := kClient.CoreV1().Secrets(ns).Delete(ctx, name, metav1.DeleteOptions{}); err != nil {
		if kerrors.IsNotFound(err) {
			p.Printfln(`Secret "%s/%s" not found`, ns, name)
			return nil
		}
		return errors.Wrapf(err, `failed to delete Secret "%s/%s"`, ns, name)
	}
	p.Printfln(`Secret "%s/%s" deleted`, ns, name)
	return nil
}

func (c *connectCmd) createRobot(ctx context.Context, p pterm.TextPrinter, kClient *kubernetes.Clientset, ar *accounts.AccountResponse, rc *robots.Client, oc *organizations.Client, u undo.Undoer, cmr **corev1.ConfigMap) (uuid.UUID, error) {
	cm := *cmr
	rs, err := oc.ListRobots(ctx, ar.Organization.ID)
	if err != nil {
		return uuid.UUID{}, errors.Wrapf(err, "failed to list Robots for %q", ar.Organization.Name)
	}
	// find an existing robot token.
	for _, r := range rs {
		if r.Name != c.Space {
			continue
		}
		p.Printfln(`Robot "%s/%s" exists`, ar.Organization.Name, c.Space)
		// delete generated robot at clean up
		u.Undo(func() error {
			return c.deleteRobot(ctx, p, ar, rc, r.ID)
		})
		// if robot is already in the configmap, return.
		if cm.Data[keyRobotID] == r.ID.String() {
			return r.ID, nil
		}
		// record the robot into the configmap.
		cm.Data[keyRobotID] = r.ID.String()
		cm, err = kClient.CoreV1().ConfigMaps(agentNs).Update(ctx, cm, metav1.UpdateOptions{})
		if err != nil {
			return uuid.UUID{}, errors.Wrapf(err, `failed to update ConfigMap "%s/%s"`, agentNs, connConfMap)
		}
		*cmr = cm
		return r.ID, nil
	}

	p.Printfln(`Creating Robot "%s/%s"`, ar.Organization.Name, c.Space)
	rr, err := rc.Create(ctx, &robots.RobotCreateParameters{
		Attributes: robots.RobotAttributes{
			Name:        c.Space,
			Description: fmt.Sprintf("Robot used for authenticating Space %q with Upbound Connect", c.Space),
		},
		Relationships: robots.RobotRelationships{
			Owner: robots.RobotOwner{
				Data: robots.RobotOwnerData{
					Type: robots.RobotOwnerOrganization,
					ID:   strconv.FormatUint(uint64(ar.Organization.ID), 10),
				},
			},
		},
	})
	if err != nil {
		return uuid.UUID{}, errors.Wrapf(err, `failed to create Robot "%s/%s""`, ar.Organization.Name, c.Space)
	}
	u.Undo(func() error {
		return c.deleteRobot(ctx, p, ar, rc, rr.ID)
	})
	p.Printfln(`Robot "%s/%s" created`, ar.Organization.Name, c.Space)
	cm.Data[keyRobotID] = rr.ID.String()
	cm, err = kClient.CoreV1().ConfigMaps(agentNs).Update(ctx, cm, metav1.UpdateOptions{})
	if err != nil {
		return uuid.UUID{}, errors.Wrapf(err, `failed to update ConfigMap "%s/%s"`, agentNs, connConfMap)
	}
	*cmr = cm
	return rr.ID, nil
}

func (c *connectCmd) deleteRobot(ctx context.Context, p pterm.TextPrinter, ar *accounts.AccountResponse, rc *robots.Client, rid uuid.UUID) error {
	p.Printfln(`Deleting Robot "%s/%s"`, ar.Organization.Name, c.Space)
	if err := rc.Delete(ctx, rid); err != nil && !sdkerrs.IsNotFound(err) {
		return errors.Wrapf(err, `failed to delete Robot "%s/%s"`, ar.Organization.Name, c.Space)
	}
	p.Printfln(`Robot "%s/%s" deleted`, ar.Organization.Name, c.Space)
	return nil
}

func (c *connectCmd) createToken(ctx context.Context, connectSpinner *pterm.SpinnerPrinter, kClient *kubernetes.Clientset, ar *accounts.AccountResponse, rc *robots.Client, tc *tokens.Client, rid uuid.UUID, u undo.Undoer, cmr **corev1.ConfigMap) (*tokens.TokenResponse, error) {
	cm := *cmr
	// try to find a pre-existing token for the space.
	trs, err := rc.ListTokens(ctx, rid)
	if err != nil {
		return nil, errors.Wrapf(err, `failed to list Tokens for Robot "%s/%s"`, ar.Organization.Name, c.Space)
	}
	for _, tr := range trs.DataSet {
		if fmt.Sprint(tr.AttributeSet["name"]) != c.Space {
			continue
		}
		connectSpinner.InfoPrinter.Printfln("Replacing Token %q", tr.ID)
		if err := tc.Delete(ctx, tr.ID); err != nil && !sdkerrs.IsNotFound(err) {
			return nil, errors.Wrapf(err, "failed to delete Token %q", tr.ID)
		}
		connectSpinner.InfoPrinter.Printfln("Token %q deleted", tr.ID)
	}
	connectSpinner.InfoPrinter.Printfln(`Creating a new Token for Robot "%s/%s"`, ar.Organization.Name, c.Space)
	// TODO(tnthornton): maybe we want to allow more than 1 token to be
	// generated for a given Space. If so, we should generate the name
	// similar to what we do with the Space name.
	tr, err := tc.Create(ctx, &tokens.TokenCreateParameters{
		Attributes: tokens.TokenAttributes{
			Name: c.Space,
		},
		Relationships: tokens.TokenRelationships{
			Owner: tokens.TokenOwner{
				Data: tokens.TokenOwnerData{
					Type: tokens.TokenOwnerRobot,
					ID:   rid.String(),
				},
			},
		},
	})
	if err != nil {
		return nil, errors.Wrapf(err, `failed to create Token for Robot "%s/%s"`, ar.Organization.Name, c.Space)
	}
	u.Undo(func() error {
		connectSpinner.InfoPrinter.Printfln("Deleting Token %q", tr.ID)
		if err := tc.Delete(ctx, tr.ID); err != nil && !sdkerrs.IsNotFound(err) {
			return errors.Wrapf(err, "failed to delete Token %q", tr.ID)
		}
		connectSpinner.InfoPrinter.Printfln("Token %q deleted", tr.ID)
		return nil
	})
	connectSpinner.InfoPrinter.Printfln("Token %q created", tr.ID)
	cm.Data[keyTokenID] = tr.ID.String()
	cm, err = kClient.CoreV1().ConfigMaps(agentNs).Update(ctx, cm, metav1.UpdateOptions{})
	if err != nil {
		return nil, errors.Wrapf(err, `failed to update ConfigMap "%s/%s"`, agentNs, connConfMap)
	}
	*cmr = cm
	return tr, nil
}

// warnAndConfirmWithSpinner prints a warning and confirmation prompt while a
// spinner is running, preventing the spinner from drawing over the
// prompt.
func warnAndConfirmWithSpinner(spinner *pterm.SpinnerPrinter, warning string, args ...any) error {
	spinner.IsActive = false
	defer func() {
		spinner.IsActive = true
	}()
	return warnAndConfirm(warning, args...)
}
