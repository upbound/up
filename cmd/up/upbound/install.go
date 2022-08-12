// Copyright 2021 Upbound Inc
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

package upbound

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/pkg/errors"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/yaml"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/fieldpath"
	"github.com/crossplane/crossplane-runtime/pkg/resource"

	"github.com/upbound/up/internal/auth"
	"github.com/upbound/up/internal/input"
	"github.com/upbound/up/internal/install"
	"github.com/upbound/up/internal/install/helm"
	"github.com/upbound/up/internal/kube"
	"github.com/upbound/up/internal/license"
)

var (
	watcherTimeout int64 = 600
)

const (
	defaultTimeout = 30 * time.Second

	defaultSecretAccessKey = "access_key"
	defaultSecretSignature = "signature"
	defaultImagePullSecret = "upbound-pull-secret"

	errReadParametersFile     = "unable to read parameters file"
	errParseInstallParameters = "unable to parse install parameters"
	errGetRegistryToken       = "failed to acquire auth token"
	errGetAccessKey           = "failed to acquire access key"
	errCreateImagePullSecret  = "failed to create image pull secret"
	errCreateLicenseSecret    = "failed to create license secret"
	errCreateNamespace        = "failed to create namespace"
)

// BeforeApply sets default values in login before assignment and validation.
func (c *installCmd) BeforeApply() error {
	c.prompter = input.NewPrompter()
	return nil
}

// AfterApply sets default values in command after assignment and validation.
func (c *installCmd) AfterApply(insCtx *install.Context) error {
	id, err := c.prompter.Prompt("License ID", false)
	if err != nil {
		return err
	}
	token, err := c.prompter.Prompt("License Key", true)
	if err != nil {
		return err
	}
	c.id = id
	c.token = token
	client, err := kubernetes.NewForConfig(insCtx.Kubeconfig)
	if err != nil {
		return err
	}
	c.kClient = client
	secret := kube.NewSecretApplicator(client)
	c.pullSecret = newImagePullApplicator(secret)
	auth := auth.NewProvider(
		auth.WithBasicAuth(id, token),
		auth.WithEndpoint(c.Registry),
		auth.WithOrgID(c.OrgID),
		auth.WithProductID(c.ProductID),
	)
	license := license.NewProvider(
		license.WithEndpoint(c.DMV),
		license.WithOrgID(c.OrgID),
		license.WithProductID(c.ProductID),
	)
	c.access = newAccessKeyApplicator(auth, license, secret)
	mgr, err := helm.NewManager(insCtx.Kubeconfig,
		upboundChart,
		c.Repo,
		helm.WithNamespace(insCtx.Namespace),
		helm.WithBasicAuth(id, token),
		helm.IsOCI(),
		helm.WithChart(c.Bundle))
	if err != nil {
		return err
	}
	c.mgr = mgr
	base := map[string]any{}
	if c.File != nil {
		defer c.File.Close() //nolint:errcheck,gosec
		b, err := io.ReadAll(c.File)
		if err != nil {
			return errors.Wrap(err, errReadParametersFile)
		}
		if err := yaml.Unmarshal(b, &base); err != nil {
			return errors.Wrap(err, errReadParametersFile)
		}
		if err := c.File.Close(); err != nil {
			return errors.Wrap(err, errReadParametersFile)
		}
	}
	c.parser = helm.NewParser(base, c.Set)
	return nil
}

// installCmd installs Upbound.
type installCmd struct {
	mgr        install.Manager
	parser     install.ParameterParser
	kClient    kubernetes.Interface
	prompter   input.Prompter
	access     *accessKeyApplicator
	pullSecret *imagePullApplicator
	id         string
	token      string

	Version string `arg:"" help:"Upbound version to install."`

	commonParams
	install.CommonParams
}

// Run executes the install command.
func (c *installCmd) Run(insCtx *install.Context) error {

	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()

	params, err := c.parser.Parse()
	if err != nil {
		return errors.Wrap(err, errParseInstallParameters)
	}

	// Create namespace if it does not exist.
	fmt.Printf("[%d/%d]: creating namespace: %s\n", 1, 6, insCtx.Namespace)
	_, err = c.kClient.CoreV1().Namespaces().Create(ctx, &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: insCtx.Namespace,
		},
	}, metav1.CreateOptions{})
	if err != nil && !kerrors.IsAlreadyExists(err) {
		return errors.Wrap(err, errCreateNamespace)
	}

	// Create or update image pull secret.
	fmt.Printf("[%d/%d]: creating secret: %s\n", 2, 6, defaultImagePullSecret)
	if err := c.pullSecret.apply(ctx, defaultImagePullSecret, insCtx.Namespace, c.id, c.token, c.Registry.String()); err != nil {
		return errors.Wrap(err, errCreateImagePullSecret)
	}

	// Create or update access key secret unless skip license is specified.
	if !c.SkipLicense {
		keyVersion := c.Version
		if c.KeyVersionOverride != "" {
			keyVersion = c.KeyVersionOverride
		}
		if err := c.access.apply(ctx, c.LicenseSecretName, insCtx.Namespace, keyVersion); err != nil {
			return errors.Wrap(err, errCreateLicenseSecret)
		}
	}

	fmt.Printf("[%d/%d]: initializing upbound\n", 3, 6)
	err = c.mgr.Install(c.Version, params)
	if err != nil {
		return err
	}
	fmt.Printf("[%d/%d]: starting upbound\n", 4, 6)

	watchCtx := context.Background()

	// change this to:
	// * watch the upbound CR
	// * output watches from across cluster
	// * add comic flag

	crdClient, err := dynamic.NewForConfig(insCtx.Kubeconfig)
	if err != nil {
		return err
	}

	crdWatcher, err := crdClient.Resource(schema.GroupVersionResource{
		Group:    upboundGroup,
		Version:  upboundVersion,
		Resource: upboundResourcePlural,
	}).Watch(watchCtx, metav1.ListOptions{TimeoutSeconds: &watcherTimeout})
	if err != nil {
		return err
	}

	for {
		event, ok := <-crdWatcher.ResultChan()
		if !ok {
			break
		}

		uu, ok := event.Object.(*unstructured.Unstructured)
		if !ok {
			continue
		}

		u := Upbound{*uu}

		switch event.Type {
		case watch.Added:
			// fmt.Printf("Upbound add: %s\n", u.GetObjectKind().GroupVersionKind())
		case watch.Modified:
			// fmt.Printf("Upbound modified: %s\n", u.GetObjectKind().GroupVersionKind())
			if resource.IsConditionTrue(u.GetCondition(xpv1.TypeReady)) {
				crdWatcher.Stop()
			}
		}
	}

	watcher, err := c.kClient.
		AppsV1().
		Deployments("").
		Watch(watchCtx, metav1.ListOptions{TimeoutSeconds: &watcherTimeout})
	if err != nil {
		return err
	}

	dstack := make(map[schema.GroupVersionKind]struct{})

	for {
		event, ok := <-watcher.ResultChan()
		if !ok {
			break
		}

		o, ok := event.Object.(*appsv1.Deployment)
		if !ok {
			continue
		}
		d := Deployment{*o}

		switch event.Type {
		case watch.Added:
			fmt.Printf("Deployment %s/%s added\n", d.Namespace, d.Name)
			dstack[d.GroupVersionKind()] = struct{}{}
		case watch.Modified:
			fmt.Printf("Deployment %s/%s modified\n", d.Namespace, d.Name)

			if resource.IsConditionTrue(d.GetCondition(appsv1.DeploymentAvailable)) {
				fmt.Printf("deployment available: %s\n", d.Name)
				fmt.Printf("stack before%+d\n", len(dstack))
				delete(dstack, d.GroupVersionKind())
				fmt.Printf("stack after%+d\n", len(dstack))
			}
		}

		if len(dstack) == 0 {
			watcher.Stop()
		}
	}

	fmt.Printf("[%d/%d]: gathering ingress information\n", 5, 6)

	fmt.Printf("[%d/%d]: upbound ready\n", 6, 6)
	return err
}

// Upbound represents the Upbound CustomResource and extends an
// unstructured.Unstructured.
type Upbound struct {
	unstructured.Unstructured
}

// GetCondition returns the condition for the given xpv1.ConditionType if it
// exists, otherwise returns nil.
func (s *Upbound) GetCondition(ct xpv1.ConditionType) xpv1.Condition {
	conditioned := xpv1.ConditionedStatus{}
	// The path is directly `status` because conditions are inline.
	if err := fieldpath.Pave(s.Object).GetValueInto("status", &conditioned); err != nil {
		return xpv1.Condition{}
	}
	return conditioned.GetCondition(ct)
}

// Deployment extends an appsv1.Deployment.
type Deployment struct {
	appsv1.Deployment
}

// GetCondition returns the condition for the given DeploymentConditionType if it
// exists, otherwise returns nil.
func (s *Deployment) GetCondition(ct appsv1.DeploymentConditionType) xpv1.Condition {
	for _, c := range s.Status.Conditions {
		if c.Type == ct {
			return xpv1.Condition{
				Type:               xpv1.ConditionType(c.Type),
				Status:             c.Status,
				LastTransitionTime: c.LastTransitionTime,
				Reason:             xpv1.ConditionReason(c.Reason),
				Message:            c.Message,
			}
		}
	}

	return xpv1.Condition{Type: xpv1.ConditionType(ct), Status: corev1.ConditionUnknown}
}
