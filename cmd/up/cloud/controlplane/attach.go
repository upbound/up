/*
Copyright 2021 Upbound Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controlplane

import (
	"context"
	"fmt"

	"github.com/alecthomas/kong"
	"github.com/google/uuid"
	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	// Allow auth to all
	_ "k8s.io/client-go/plugin/pkg/client/auth"

	cp "github.com/upbound/up-sdk-go/service/controlplanes"
	"github.com/upbound/up-sdk-go/service/tokens"

	"github.com/upbound/up/internal/cloud"
	"github.com/upbound/up/internal/kube"
)

const (
	kubeIDNamespace = "kube-system"
	jwtKey          = "jwt"

	errKubeSystemUID = "unable to extract kube-system namespace uid for usage as cluster identifier"
	errNoToken       = "could not identify token in response"
)

// AfterApply sets default values in command after assignment and validation.
func (c *AttachCmd) AfterApply() error {
	if c.KubeClusterID == uuid.Nil {
		config, err := kube.GetKubeConfig(c.Kubeconfig)
		if err != nil {
			return err
		}
		client, err := kubernetes.NewForConfig(config)
		if err != nil {
			return err
		}
		c.kClient = client
	}
	return nil
}

// AttachCmd adds a user or token profile with session token to the up config
// file.
type AttachCmd struct {
	kClient kubernetes.Interface

	Name string `arg:"" required:"" help:"Name of control plane."`

	Description   string    `short:"d" help:"Description for control plane."`
	KubeClusterID uuid.UUID `help:"ID for self-hosted Kubernetes cluster."`
	Kubeconfig    string    `type:"existingfile" help:"Override default kubeconfig path."`
	ViewOnly      bool      `help:"Create control plane with view only permissions."`
}

// Run executes the attach command.
func (c *AttachCmd) Run(kong *kong.Context, client *cp.Client, token *tokens.Client, cloudCtx *cloud.Context) error {
	if c.KubeClusterID == uuid.Nil {
		ns, err := c.kClient.CoreV1().Namespaces().Get(context.Background(), kubeIDNamespace, metav1.GetOptions{})
		if err != nil {
			return errors.Wrap(err, errKubeSystemUID)
		}
		c.KubeClusterID, err = uuid.Parse(string(ns.GetObjectMeta().GetUID()))
		if err != nil {
			return errors.Wrap(err, errKubeSystemUID)
		}
	}
	cpRes, err := client.Create(context.Background(), &cp.ControlPlaneCreateParameters{
		Account:       cloudCtx.Account,
		Name:          c.Name,
		Description:   c.Description,
		SelfHosted:    true,
		KubeClusterID: c.KubeClusterID.String(),
	})
	if err != nil {
		return err
	}
	if c.ViewOnly {
		if err := client.SetViewOnly(context.Background(), cpRes.ControlPlane.ID, c.ViewOnly); err != nil {
			return err
		}
	}
	tRes, err := token.Create(context.Background(), &tokens.TokenCreateParameters{
		Attributes: tokens.TokenAttributes{
			Name: c.Name,
		},
		Relationships: tokens.TokenRelationships{
			Owner: tokens.TokenOwner{
				Data: tokens.TokenOwnerData{
					Type: tokens.TokenOwnerControlPlane,
					ID:   cpRes.ControlPlane.ID,
				},
			},
		},
	})
	if err != nil {
		return err
	}
	jwt, ok := tRes.DataSet.Meta[jwtKey]
	if !ok {
		return errors.New(errNoToken)
	}
	fmt.Fprintf(kong.Stdout, "%s\n", jwt)
	return err
}
