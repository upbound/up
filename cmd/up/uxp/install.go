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
	"io"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/pterm/pterm"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/yaml"

	"github.com/upbound/up/internal/install"
	"github.com/upbound/up/internal/install/helm"
)

const (
	errReadParametersFile     = "unable to read parameters file"
	errParseInstallParameters = "unable to parse install parameters"
)

// AfterApply sets default values in command after assignment and validation.
func (c *installCmd) AfterApply(insCtx *install.Context) error {
	repo := RepoURL
	if c.Unstable {
		repo = uxpUnstableRepoURL
	}
	mgr, err := helm.NewManager(insCtx.Kubeconfig,
		chartName,
		repo,
		helm.WithNamespace(insCtx.Namespace),
		helm.WithChart(c.Bundle),
		helm.WithAlternateChart(alternateChartName))
	if err != nil {
		return err
	}
	c.mgr = mgr
	client, err := kubernetes.NewForConfig(insCtx.Kubeconfig)
	if err != nil {
		return err
	}
	c.kClient = client
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

// installCmd installs UXP.
type installCmd struct {
	mgr     install.Manager
	parser  install.ParameterParser
	kClient kubernetes.Interface

	Version  string `arg:"" optional:"" help:"UXP version to install."`
	Unstable bool   `help:"Allow installing unstable versions."`

	install.CommonParams
}

// Run executes the install command.
func (c *installCmd) Run(p pterm.TextPrinter, insCtx *install.Context) error {
	// Create namespace if it does not exist.
	_, err := c.kClient.CoreV1().Namespaces().Create(context.Background(), &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: insCtx.Namespace,
		},
	}, metav1.CreateOptions{})
	if err != nil && !kerrors.IsAlreadyExists(err) {
		return err
	}
	params, err := c.parser.Parse()
	if err != nil {
		return errors.Wrap(err, errParseInstallParameters)
	}
	if err = c.mgr.Install(c.Version, params); err != nil {
		return err
	}

	curVer, err := c.mgr.GetCurrentVersion()
	if err != nil {
		return err
	}
	p.Printfln("UXP %s installed", curVer)
	return nil
}
