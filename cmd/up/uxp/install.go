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

package uxp

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/upbound/up/internal/uxp"
	"github.com/upbound/up/internal/uxp/installers/helm"
)

// AfterApply sets default values in command after assignment and validation.
func (c *installCmd) AfterApply(uxpCtx *uxp.Context) error {
	installer, err := helm.NewInstaller(uxpCtx.Kubeconfig,
		helm.WithNamespace(uxpCtx.Namespace),
		helm.AllowUnstableVersions(c.Unstable))
	if err != nil {
		return err
	}
	c.installer = installer
	client, err := kubernetes.NewForConfig(uxpCtx.Kubeconfig)
	if err != nil {
		return err
	}
	c.kClient = client
	return nil
}

// installCmd installs UXP.
type installCmd struct {
	installer uxp.Installer
	kClient   kubernetes.Interface

	Version string `arg:"" optional:"" help:"UXP version to install."`

	Unstable bool `help:"Allow installing unstable UXP versions."`
}

// Run executes the install command.
func (c *installCmd) Run(uxpCtx *uxp.Context) error {
	// Create namespace if it does not exist.
	_, err := c.kClient.CoreV1().Namespaces().Create(context.Background(), &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: uxpCtx.Namespace,
		},
	}, metav1.CreateOptions{})
	if err != nil && !kerrors.IsAlreadyExists(err) {
		return err
	}
	err = c.installer.Install(c.Version)
	if err != nil {
		return err
	}
	return nil
}
