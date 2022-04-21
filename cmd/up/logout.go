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

package main

import (
	"context"
	"net/http"
	"net/url"

	"github.com/alecthomas/kong"
	"github.com/pkg/errors"

	"github.com/upbound/up-sdk-go"

	"github.com/upbound/up/internal/config"
	"github.com/upbound/up/internal/upbound"
)

const (
	logoutPath = "/v1/logout"

	errLogoutFailed      = "unable to logout"
	errGetProfile        = "failed to get profile"
	errRemoveTokenFailed = "failed to remove token"
)

// AfterApply sets default values in login after assignment and validation.
func (c *logoutCmd) AfterApply(kongCtx *kong.Context) error {
	src := config.NewFSSource()
	if err := src.Initialize(); err != nil {
		return err
	}
	conf, err := config.Extract(src)
	if err != nil {
		return err
	}
	upCtx := &upbound.Context{
		Profile:  c.Profile,
		Account:  c.Account,
		Endpoint: c.Endpoint,
		Cfg:      conf,
		CfgSrc:   src,
	}
	var profile config.Profile
	var name string
	if upCtx.Profile == "" {
		name, profile, err = upCtx.Cfg.GetDefaultUpboundProfile()
		if err != nil {
			return err
		}
		upCtx.Profile = name
		upCtx.ID = profile.ID
	} else {
		profile, err = upCtx.Cfg.GetUpboundProfile(upCtx.Profile)
		if err != nil {
			return err
		}
	}
	// If account has not already been set, use the profile default.
	if upCtx.Account == "" {
		upCtx.Account = profile.Account
	}
	// If no account is set in profile, return an error.
	if upCtx.Account == "" {
		return errors.New(errNoAccount)
	}
	kongCtx.Bind(upCtx)
	cfg, err := upbound.BuildSDKConfig(profile.Session, c.Endpoint)
	if err != nil {
		return err
	}
	c.client = cfg.Client
	return nil
}

// logoutCmd invalidates a stored session token for a given profile.
type logoutCmd struct {
	client up.Client

	// Common Upbound API configuration
	Endpoint *url.URL `env:"UP_ENDPOINT" default:"https://api.upbound.io" help:"Endpoint used for Upbound API."`
	Profile  string   `env:"UP_PROFILE" help:"Profile used to execute command."`
	Account  string   `short:"a" env:"UP_ACCOUNT" help:"Account used to execute command."`
}

// Run executes the logout command.
func (c *logoutCmd) Run(upCtx *upbound.Context) error {
	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()
	req, err := c.client.NewRequest(ctx, http.MethodPost, logoutPath, "", nil)
	if err != nil {
		return errors.Wrap(err, errLogoutFailed)
	}
	if err := c.client.Do(req, nil); err != nil {
		return errors.Wrap(err, errLogoutFailed)
	}

	// Logout is successful, remove token from config and update.
	profile, err := upCtx.Cfg.GetUpboundProfile(upCtx.Profile)
	if err != nil {
		return errors.Wrap(err, errGetProfile)
	}
	profile.Session = ""
	if err := upCtx.Cfg.AddOrUpdateUpboundProfile(upCtx.Profile, profile); err != nil {
		return errors.Wrap(err, errRemoveTokenFailed)
	}
	return errors.Wrap(upCtx.CfgSrc.UpdateConfig(upCtx.Cfg), errUpdateConfig)
}
