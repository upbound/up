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
	"net/url"
	"strconv"

	"github.com/alecthomas/kong"
	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/google/uuid"
	"github.com/pterm/pterm"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apiserver/pkg/storage/names"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"

	"github.com/upbound/up-sdk-go/service/accounts"
	"github.com/upbound/up-sdk-go/service/organizations"
	"github.com/upbound/up-sdk-go/service/robots"
	"github.com/upbound/up-sdk-go/service/tokens"
	"github.com/upbound/up/cmd/up/robot"
	"github.com/upbound/up/cmd/up/robot/token"
	"github.com/upbound/up/cmd/up/space/prerequisites"
	"github.com/upbound/up/internal/config"
	"github.com/upbound/up/internal/install"
	"github.com/upbound/up/internal/install/helm"
	"github.com/upbound/up/internal/upbound"
	"github.com/upbound/up/internal/upterm"
)

const (
	agentChart = "agent"

	// TODO(tnthornton) these can probably be replaced by our public chart
	// museum. This would allow us to use wildcards like mxp-connector.
	supportedVersion = "0.0.0-100.g216e157"
	agentRegistry    = "us-west1-docker.pkg.dev/orchestration-build/connect"

	// TODO(tnthornton) maybe move this to the agent chart?
	devConnectURL = "nats://connect.u5d.dev"
)

type attachCmd struct {
	Upbound upbound.Flags     `embed:""`
	Kube    upbound.KubeFlags `embed:""`

	helmMgr install.Manager
	prereqs *prerequisites.Manager
	parser  install.ParameterParser
	kClient kubernetes.Interface
	dClient dynamic.Interface
	quiet   config.QuietFlag

	ng names.NameGenerator

	Space string `arg:"" optional:"" help:"Name of the Upbound Space. If name is not a supplied, one is generated."`
	Token string `name:"robot-token" optional:"" help:"The Upbound robot token contents used to authenticate the connection."`
}

func (c *attachCmd) AfterApply(kongCtx *kong.Context) error {
	registryURL, err := url.Parse(agentRegistry)
	if err != nil {
		return err
	}

	upCtx, err := upbound.NewFromFlags(c.Upbound)
	if err != nil {
		return err
	}
	cfg, err := upCtx.BuildSDKConfig()
	if err != nil {
		return err
	}

	kongCtx.Bind(upCtx)
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
	c.kClient = kClient

	dClient, err := dynamic.NewForConfig(kubeconfig)
	if err != nil {
		return err
	}
	c.dClient = dClient
	mgr, err := helm.NewManager(kubeconfig,
		agentChart,
		registryURL,
		helm.WithNamespace("upbound-connect"),
		helm.CreateNamespace(true),
		helm.IsOCI(),
		helm.Wait(),
	)
	if err != nil {
		return err
	}
	c.helmMgr = mgr

	c.ng = names.SimpleNameGenerator

	return nil
}

// Run executes the install command.
func (c *attachCmd) Run(ctx context.Context, p pterm.TextPrinter, upCtx *upbound.Context, ac *accounts.Client, oc *organizations.Client, tc *tokens.Client, rc *robots.Client) error {
	if c.Space == "" {
		c.Space = c.ng.GenerateName("space-")
	}
	fmt.Printf("Using Space name: %s\n", c.Space)

	if c.Token == "" {

		p.Println("Generating new Robot and Token to authenticate agent...")
		a, err := ac.Get(ctx, upCtx.Account)
		if err != nil {
			return err
		}
		if a.Account.Type != accounts.AccountOrganization {
			return errors.New(robot.ErrUserAccount)
		}

		if err := c.createRobot(ctx, upCtx, p, a, ac, rc); err != nil {
			return err
		}

		res, err := c.createToken(ctx, upCtx, p, a, oc, tc)
		if err != nil {
			return err
		}
		c.Token = fmt.Sprint(res.Meta["jwt"])
	}

	attachSpinner, _ := upterm.CheckmarkSuccessSpinner.Start("Installing agent to connect to Upbound Console...")

	if err := c.createNamespace(ctx, "upbound-connect"); err != nil {
		return err
	}
	if err := c.createTokenSecret(ctx, "space-token", "upbound-connect", c.Token); err != nil {
		return err
	}

	if err := c.helmMgr.Install(supportedVersion, map[string]any{
		"nats": map[string]any{
			"url": devConnectURL,
		},
		"space":       c.Space,
		"account":     upCtx.Account,
		"tokenSecret": "space-token",
	}); err != nil {
		return err
	}

	attachSpinner.Success()
	return nil
}

func (c *attachCmd) createNamespace(ctx context.Context, ns string) error {
	_, err := c.kClient.CoreV1().Namespaces().Create(ctx, &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: ns,
		},
	}, metav1.CreateOptions{})
	if err != nil && !kerrors.IsAlreadyExists(err) {
		return errors.Wrap(err, fmt.Sprintf(errFmtCreateNamespace, ns))
	}
	return nil
}

func (c *attachCmd) createTokenSecret(ctx context.Context, name, ns, token string) error {
	s := &corev1.Secret{}
	s.SetName(name)
	s.Data = map[string][]byte{
		"token": []byte(c.Token),
	}
	_, err := c.kClient.CoreV1().Secrets(ns).Create(ctx, s, metav1.CreateOptions{})
	if err != nil {
		return err
	}

	return nil
}

func (c *attachCmd) createRobot(ctx context.Context, upCtx *upbound.Context, p pterm.TextPrinter, ar *accounts.AccountResponse, ac *accounts.Client, rc *robots.Client) error {
	if _, err := rc.Create(ctx, &robots.RobotCreateParameters{
		Attributes: robots.RobotAttributes{
			Name:        c.Space,
			Description: fmt.Sprintf("Robot used for authenticating Space %s with Upbound Connect", c.Space),
		},
		Relationships: robots.RobotRelationships{
			Owner: robots.RobotOwner{
				Data: robots.RobotOwnerData{
					Type: robots.RobotOwnerOrganization,
					ID:   strconv.FormatUint(uint64(ar.Organization.ID), 10),
				},
			},
		},
	}); err != nil {
		return err
	}

	p.Printfln("Robot %s/%s created", upCtx.Account, c.Space)
	return nil
}

func (c *attachCmd) createToken(ctx context.Context, upCtx *upbound.Context, p pterm.TextPrinter, ar *accounts.AccountResponse, oc *organizations.Client, tc *tokens.Client) (*tokens.TokenResponse, error) {
	// NOTE(tnthornton): the API does not yet support a way to get a Robot via
	// it's name, only its ID. Resorting to ListRobots like we do in other
	// places until this is sorted out.
	rs, err := oc.ListRobots(ctx, ar.Organization.ID)
	if err != nil {
		return nil, err
	}
	if len(rs) == 0 {
		return nil, errors.Errorf(token.ErrFindRobotFmt, c.Space, upCtx.Account)
	}
	// TODO(tnthornton): because this API does not guarantee name uniqueness, we
	// must guarantee that exactly one robot exists in the specified account
	// with the provided name. Logic should be simplified when the API is
	// updated.
	var id uuid.UUID
	found := false
	for _, r := range rs {
		if r.Name == c.Space {
			if found {
				return nil, errors.Errorf(token.ErrMultipleRobotFmt, c.Space, upCtx.Account)
			}
			id = r.ID
			found = true
		}
	}
	if !found {
		return nil, errors.Errorf(token.ErrFindRobotFmt, c.Space, upCtx.Account)
	}
	// TODO(tnthornton): maybe we want to allow more than 1 token to be
	// generated for a given Space. If so, we should generate the name
	// similar to what we do with the Space name.
	res, err := tc.Create(ctx, &tokens.TokenCreateParameters{
		Attributes: tokens.TokenAttributes{
			Name: c.Space,
		},
		Relationships: tokens.TokenRelationships{
			Owner: tokens.TokenOwner{
				Data: tokens.TokenOwnerData{
					Type: tokens.TokenOwnerRobot,
					ID:   id.String(),
				},
			},
		},
	})
	if err != nil {
		return nil, err
	}
	p.Printfln("Token %s/%s/%s created", upCtx.Account, c.Space, c.Space)
	return res, nil
}
