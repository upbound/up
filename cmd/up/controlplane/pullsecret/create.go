// Copyright 2022 Upbound Inc
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

package pullsecret

import (
	"context"
	"fmt"

	"github.com/alecthomas/kong"
	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/pterm/pterm"
	"k8s.io/client-go/kubernetes"

	"github.com/upbound/up/internal/kube"
	"github.com/upbound/up/internal/upbound"
)

const defaultUsername = "_token"

const (
	errMissingProfileCreds = "current profile does not contain credentials"
	errCreatePullSecret    = "failed to create package pull secret"
)

// AfterApply constructs and binds Upbound-specific context to any subcommands
// that have Run() methods that receive it.
func (c *createCmd) AfterApply(kongCtx *kong.Context, upCtx *upbound.Context) error {
	if upCtx.Profile.IsSpace() {
		return fmt.Errorf("create is not supported for space profile %q", upCtx.ProfileName)
	}

	kubeconfig, err := kube.GetKubeConfig(c.Kubeconfig)
	if err != nil {
		return err
	}
	if upCtx.WrapTransport != nil {
		kubeconfig.Wrap(upCtx.WrapTransport)
	}
	client, err := kubernetes.NewForConfig(kubeconfig)
	if err != nil {
		return err
	}
	c.kClient = client
	if c.File != "" {
		tf, err := upbound.TokenFromPath(c.File)
		if err != nil {
			return err
		}
		c.user, c.pass = tf.AccessID, tf.Token
	}
	if c.user == "" || c.pass == "" {
		if upCtx.Profile.Session == "" {
			return errors.New(errMissingProfileCreds)
		}
		c.user, c.pass = defaultUsername, upCtx.Profile.Session
		pterm.Warning.WithWriter(kongCtx.Stdout).Printfln("Using temporary user credentials that will expire within 30 days.")
	}
	return nil
}

// createCmd creates a package pull secret.
type createCmd struct {
	kClient kubernetes.Interface
	user    string
	pass    string

	Name string `arg:"" default:"package-pull-secret" help:"Name of the pull secret."`

	// NOTE(hasheddan): kong automatically cleans paths tagged with existingfile.
	File       string `type:"existingfile" short:"f" help:"Path to credentials file. Credentials from profile are used if not specified."`
	Kubeconfig string `type:"existingfile" help:"Override default kubeconfig path."`
	Namespace  string `short:"n" env:"UPBOUND_NAMESPACE" default:"upbound-system" help:"Kubernetes namespace for pull secret."`
}

// Run executes the pull secret command.
func (c *createCmd) Run(ctx context.Context, p pterm.TextPrinter, upCtx *upbound.Context) error { //nolint:gocyclo
	if err := kube.NewImagePullApplicator(kube.NewSecretApplicator(c.kClient)).
		Apply(ctx,
			c.Name,
			c.Namespace,
			c.user,
			c.pass,
			upCtx.RegistryEndpoint.Hostname(),
		); err != nil {
		return errors.Wrap(err, errCreatePullSecret)
	}

	p.Printfln("%s/%s created", c.Namespace, c.Name)
	return nil
}
