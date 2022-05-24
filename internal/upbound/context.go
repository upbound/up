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
	"net/http"
	"net/http/cookiejar"
	"net/url"

	"github.com/upbound/up-sdk-go"
	"github.com/upbound/up/internal/config"
)

const (
	// UserAgent is the default user agent to use to make requests to the
	// Upbound API.
	UserAgent = "up-cli"
	// CookieName is the default cookie name used to identify a session token.
	CookieName = "SID"

	// Default API subdomain.
	apiSubdomain = "api."
	// Default registry subdomain.
	xpkgSubdomain = "xpkg."
)

// Flags are common flags used by commands that interact with Upbound.
type Flags struct {
	// Optional
	Domain  *url.URL `env:"UP_DOMAIN" default:"https://upbound.io" help:"Root Upbound domain."`
	Profile string   `env:"UP_PROFILE" help:"Profile used to execute command."`
	Account string   `short:"a" env:"UP_ACCOUNT" help:"Account used to execute command."`

	APIEndpoint      *url.URL `env:"OVERRIDE_API_ENDPOINT" hidden:"" name:"override-api-endpoint" help:"Overrides the default API endpoint."`
	RegistryEndpoint *url.URL `env:"OVERRIDE_REGISTRY_ENDPOINT" hidden:"" name:"override-registry-endpoint" help:"Overrides the default registry endpoint."`
}

// Context includes common data that Upbound consumers may utilize.
type Context struct {
	Profile          config.Profile
	Token            string
	Account          string
	Domain           *url.URL
	APIEndpoint      *url.URL
	RegistryEndpoint *url.URL
	Cfg              *config.Config
	CfgSrc           config.Source
}

// NewFromFlags constructs a new context from flags.
func NewFromFlags(f Flags) (*Context, error) {
	src := config.NewFSSource()
	if err := src.Initialize(); err != nil {
		return nil, err
	}
	conf, err := config.Extract(src)
	if err != nil {
		return nil, err
	}

	c := &Context{
		Account: f.Account,
		Domain:  f.Domain,
		Cfg:     conf,
		CfgSrc:  src,
	}

	c.APIEndpoint = f.APIEndpoint
	if c.APIEndpoint == nil {
		u := *c.Domain
		u.Host = apiSubdomain + u.Host
		c.APIEndpoint = &u
	}

	c.RegistryEndpoint = f.RegistryEndpoint
	if c.RegistryEndpoint == nil {
		u := *c.Domain
		u.Host = xpkgSubdomain + u.Host
		c.RegistryEndpoint = &u
	}

	// If profile identifier is not provided, use the default, or empty if the
	// default cannot be obtained.
	c.Profile = config.Profile{}
	if f.Profile == "" {
		if _, p, err := c.Cfg.GetDefaultUpboundProfile(); err == nil {
			c.Profile = p
		}
	} else {
		if p, err := c.Cfg.GetUpboundProfile(f.Profile); err == nil {
			c.Profile = p
		}
	}

	// If account has not already been set, use the profile default.
	if c.Account == "" {
		c.Account = c.Profile.Account
	}

	return c, nil
}

// BuildSDKConfig builds an Upbound SDK config suitable for usage with any
// service client.
func (c *Context) BuildSDKConfig(session string) (*up.Config, error) {
	cj, err := cookiejar.New(nil)
	if err != nil {
		return nil, err
	}
	cj.SetCookies(c.APIEndpoint, []*http.Cookie{{
		Name:  CookieName,
		Value: session,
	},
	})
	client := up.NewClient(func(u *up.HTTPClient) {
		u.BaseURL = c.APIEndpoint
		u.HTTP = &http.Client{
			Jar: cj,
		}
		u.UserAgent = UserAgent
	})
	return up.NewConfig(func(conf *up.Config) {
		conf.Client = client
	}), nil
}
