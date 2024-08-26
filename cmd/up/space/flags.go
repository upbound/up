// Copyright 2023 Upbound Inc.
// All rights reserved

package space

import (
	"net/url"
	"os"

	"github.com/crossplane/crossplane-runtime/pkg/errors"

	"github.com/upbound/up/internal/input"
	"github.com/upbound/up/internal/upbound"
)

type registryFlags struct {
	Repository *url.URL `hidden:"" name:"registry-repository" env:"UPBOUND_REGISTRY" default:"xpkg.upbound.io/spaces-artifacts" help:"Set registry for where to pull OCI artifacts from. This is an OCI registry reference, i.e. a URL without the scheme or protocol prefix."`
	Endpoint   *url.URL `hidden:"" name:"registry-endpoint" env:"UPBOUND_REGISTRY_ENDPOINT" default:"https://xpkg.upbound.io" help:"Set registry endpoint, including scheme, for authentication."`
}

type authorizedRegistryFlags struct {
	registryFlags

	TokenFile *os.File `name:"token-file" help:"File containing authentication token. Expecting a JSON file with \"accessId\" and \"token\" keys."`
	Username  string   `hidden:"" name:"registry-username" help:"Set the registry username."`
	Password  string   `hidden:"" name:"registry-password" help:"Set the registry password."`
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

	tf, err := upbound.TokenFromPath(p.TokenFile.Name())
	if err != nil {
		return err
	}
	p.Username, p.Password = tf.AccessID, tf.Token

	return nil
}
