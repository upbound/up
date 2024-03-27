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
	"strings"

	"github.com/alecthomas/kong"
	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/google/uuid"
	"github.com/pterm/pterm"
	"helm.sh/helm/v3/pkg/storage/driver"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	sdkerrs "github.com/upbound/up-sdk-go/errors"
	"github.com/upbound/up-sdk-go/service/robots"
	"github.com/upbound/up-sdk-go/service/spaces"
	"github.com/upbound/up/internal/install/helm"
	"github.com/upbound/up/internal/upbound"
	"github.com/upbound/up/internal/upterm"
)

type detachCmd struct {
	Upbound upbound.Flags     `embed:""`
	Kube    upbound.KubeFlags `embed:""`
}

func (c *detachCmd) AfterApply(kongCtx *kong.Context) error {
	registryURL, err := url.Parse(agentRegistry)
	if err != nil {
		return err
	}

	if err := c.Kube.AfterApply(); err != nil {
		return err
	}

	// NOTE(tnthornton) we currently only have support for stylized output.
	pterm.EnableStyling()
	upterm.DefaultObjPrinter.Pretty = true

	upCtx, err := upbound.NewFromFlags(c.Upbound)
	if err != nil {
		return err
	}
	cfg, err := upCtx.BuildSDKConfig()
	if err != nil {
		return err
	}
	kongCtx.Bind(upCtx)
	kongCtx.Bind(robots.NewClient(cfg))
	kongCtx.Bind(spaces.NewClient(cfg))

	kubeconfig := c.Kube.GetConfig()

	kClient, err := kubernetes.NewForConfig(kubeconfig)
	if err != nil {
		return err
	}
	kongCtx.Bind(kClient)

	with := []helm.InstallerModifierFn{
		helm.WithNamespace(agentNs),
		helm.IsOCI(),
	}

	mgr, err := helm.NewManager(kubeconfig,
		agentChart,
		registryURL,
		with...,
	)
	if err != nil {
		return err
	}
	kongCtx.Bind(mgr)

	return nil
}

// Run executes the detach command.
func (c *detachCmd) Run(ctx context.Context, kClient *kubernetes.Clientset, mgr *helm.Installer, sc *spaces.Client, rc *robots.Client) (rErr error) {
	detachSpinner, err := upterm.CheckmarkSuccessSpinner.Start("Disconnecting Space from Upbound Console...")
	if err != nil {
		return err
	}
	defer func() {
		if rErr != nil {
			detachSpinner.Fail(rErr)
		}
	}()
	if err := c.deleteResources(ctx, detachSpinner.InfoPrinter, kClient, rc, sc); err != nil {
		return err
	}
	detachSpinner.InfoPrinter.Println("Uninstalling connect agent...")
	if err := mgr.Uninstall(); err != nil && !errors.Is(err, driver.ErrReleaseNotFound) {
		return errors.Wrapf(err, `failed to uninstall Chart "%s/%s"`, agentNs, agentChart)
	}
	if err := deleteTokenSecret(ctx, detachSpinner.InfoPrinter, kClient, agentNs, agentSecret); err != nil && !kerrors.IsNotFound(err) {
		return err
	}
	detachSpinner.Success("Space has been successfully disconnected from Upbound Console")
	return nil
}

func (c *detachCmd) deleteResources(ctx context.Context, p pterm.TextPrinter, kClient *kubernetes.Clientset, rc *robots.Client, sc *spaces.Client) error {
	cm, err := getConnectConfigmap(ctx, kClient, agentNs, connConfMap)
	if kerrors.IsNotFound(err) {
		return nil
	}
	if err != nil {
		return errors.Wrapf(err, `failed to get ConfigMap "%s/%s"`, agentNs, agentSecret)
	}
	p.Printfln(`ConfigMap "%s/%s" found, deleting resources in Upbound Console...`, agentNs, agentSecret)
	if err := c.deleteAgentRobot(ctx, p, kClient, rc, &cm); err != nil {
		return err
	}
	if err := c.deleteGeneratedSpace(ctx, p, kClient, sc, &cm); err != nil {
		return err
	}
	if err := deleteConnectConfigmap(ctx, p, kClient, agentNs, connConfMap); err != nil {
		return err
	}
	return nil
}

func (c *detachCmd) deleteAgentRobot(ctx context.Context, p pterm.TextPrinter, kClient *kubernetes.Clientset, rc *robots.Client, cmr **corev1.ConfigMap) error {
	cm := *cmr
	v, ok := cm.Data[keyRobotID]
	if !ok {
		return nil
	}
	rid, err := uuid.Parse(v)
	if err != nil {
		return errors.Wrapf(err, "invalid robot id %q", v)
	}
	p.Printfln("Deleting Robot %q", rid)
	if err := rc.Delete(ctx, rid); err != nil && !sdkerrs.IsNotFound(err) {
		return errors.Wrapf(err, "failed to delete Robot %q", rid)
	}
	delete(cm.Data, keyRobotID)
	delete(cm.Data, keyTokenID)
	cm, err = kClient.CoreV1().ConfigMaps(agentNs).Update(ctx, cm, metav1.UpdateOptions{})
	if err != nil {
		return errors.Wrapf(err, `failed to update ConfigMap "%s/%s"`, agentNs, connConfMap)
	}
	*cmr = cm
	return nil
}

func (c *detachCmd) deleteGeneratedSpace(ctx context.Context, p pterm.TextPrinter, kClient *kubernetes.Clientset, sc *spaces.Client, cmr **corev1.ConfigMap) error {
	cm := *cmr
	v, ok := cm.Data[keySpace]
	if !ok {
		return nil
	}
	parts := strings.Split(v, "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return fmt.Errorf("invalid space %q", v)
	}
	p.Printfln("Deleting Space %q", v)
	ns, name := parts[0], parts[1]
	if err := sc.Delete(ctx, ns, name, nil); err != nil && !kerrors.IsNotFound(err) {
		return errors.Wrapf(err, `failed to delete Space "%s/%s"`, ns, name)
	}
	delete(cm.Data, keySpace)
	cm, err := kClient.CoreV1().ConfigMaps(agentNs).Update(ctx, cm, metav1.UpdateOptions{})
	if err != nil {
		return errors.Wrapf(err, `failed to update ConfigMap "%s/%s"`, agentNs, connConfMap)
	}
	*cmr = cm
	return nil
}
