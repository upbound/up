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

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"

	"github.com/upbound/up/internal/install"
)

const (
	upboundGroup          = "distribution.upbound.io"
	upboundVersion        = "v1alpha1"
	upboundResourcePlural = "upbounds"
)

// AfterApply sets default values in command after assignment and validation.
func (c *uninstallCmd) AfterApply(insCtx *install.Context) error {
	client, err := dynamic.NewForConfig(insCtx.Kubeconfig)
	if err != nil {
		return err
	}
	c.kClient = client
	return nil
}

// uninstallCmd uninstalls Upbound.
type uninstallCmd struct {
	kClient dynamic.Interface

	Name string `arg:"" optional:"" default:"install" help:"Name of Upbound install."`
}

// Run executes the uninstall command.
func (c *uninstallCmd) Run(insCtx *install.Context) error {
	return c.kClient.Resource(schema.GroupVersionResource{
		Group:    upboundGroup,
		Version:  upboundVersion,
		Resource: upboundResourcePlural,
	}).Delete(context.Background(), c.Name, metav1.DeleteOptions{})
}
