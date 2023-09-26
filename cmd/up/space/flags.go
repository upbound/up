// Copyright 2023 Upbound Inc.
// All rights reserved

package space

import (
	"io"
	"net/url"
	"os"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"k8s.io/client-go/rest"

	"github.com/upbound/up/internal/input"
	"github.com/upbound/up/internal/kube"
)

type kubeFlags struct {
	Kubeconfig string `type:"existingfile" help:"Override default kubeconfig path."`

	// set by AfterApply
	config *rest.Config
}

type registryFlags struct {
	Repository *url.URL `hidden:"" name:"registry-repository" env:"UPBOUND_REGISTRY" default:"us-west1-docker.pkg.dev/orchestration-build/upbound-environments" help:"Set registry for where to pull OCI artifacts from. This is an OCI registry reference, i.e. a URL without the scheme or protocol prefix."`
	Endpoint   *url.URL `hidden:"" name:"registry-endpoint" env:"UPBOUND_REGISTRY_ENDPOINT" default:"https://us-west1-docker.pkg.dev" help:"Set registry endpoint, including scheme, for authentication."`
}

type authorizedRegistryFlags struct {
	registryFlags

	TokenFile *os.File `name:"token-file" help:"File containing authentication token."`
	Username  string   `hidden:"" name:"registry-username" env:"UPBOUND_REGISTRY_USERNAME" help:"Set the registry username."`
	Password  string   `hidden:"" name:"registry-password" env:"UPBOUND_REGISTRY_PASSWORD" help:"Set the registry password."`
}

func (f *kubeFlags) AfterApply() error {
	restConfig, err := kube.GetKubeConfig(f.Kubeconfig)
	if err != nil {
		return err
	}
	f.config = restConfig

	return nil
}

func (p *authorizedRegistryFlags) AfterApply() error {
	if p.TokenFile == nil && p.Username == "" && p.Password == "" {
		if p.Repository.String() == defaultRegistry {
			return errors.New("--token-file is required")
		}

		prompter := input.NewPrompter()
		id, err := prompter.Prompt("Username", false)
		if err != nil {
			return err
		}
		token, err := prompter.Prompt("Password", true)
		if err != nil {
			return err
		}
		p.Username = id
		p.Password = token

		return nil
	}

	if p.Username != "" {
		return nil
	}

	b, err := io.ReadAll(p.TokenFile)
	defer p.TokenFile.Close() // nolint:errcheck
	if err != nil {
		return errors.Wrap(err, errReadTokenFile)
	}
	p.Username = jsonKey
	p.Password = string(b)

	return nil
}
