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
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/cookiejar"
	"net/url"

	"github.com/alecthomas/kong"
	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/spf13/afero"
	"k8s.io/apimachinery/pkg/types"
	utilnet "k8s.io/apimachinery/pkg/util/net"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	"k8s.io/client-go/transport"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/upbound/up-sdk-go"

	upboundv1alpha1 "github.com/upbound/up-sdk-go/apis/upbound/v1alpha1"
	"github.com/upbound/up/internal/config"
	"github.com/upbound/up/internal/profile"
)

const (
	// UserAgent is the default user agent to use to make requests to the
	// Upbound API.
	UserAgent = "up-cli"
	// CookieName is the default cookie name used to identify a session token.
	CookieName = "SID"

	// Default API subdomain.
	apiSubdomain = "api."
	// Default auth subdomain.
	authSubdomain = "auth."
	// Default proxy subdomain.
	proxySubdomain = "proxy."

	// Base path for proxy.
	proxyPath = "/v1/controlPlanes"
	// Base path for all controller client requests.
	controllerClientPath = "/apis"

	// Default registry subdomain.
	xpkgSubdomain = "xpkg."
)

const (
	errProfileNotFoundFmt = "profile not found with identifier: %s"
)

// Context includes common data that Upbound consumers may utilize.
type Context struct {
	// Profile fields
	ProfileName string
	Profile     profile.Profile
	Token       string
	Cfg         *config.Config
	CfgSrc      config.Source
	Account     string

	// Kubeconfig fields
	Kubecfg clientcmd.ClientConfig

	// Upbound API connection URLs
	Domain                *url.URL
	APIEndpoint           *url.URL
	AuthEndpoint          *url.URL
	ProxyEndpoint         *url.URL
	RegistryEndpoint      *url.URL
	InsecureSkipTLSVerify bool

	// Debugging
	DebugLevel    int
	WrapTransport func(rt http.RoundTripper) http.RoundTripper

	// Miscellaneous
	allowMissingProfile bool
	cfgPath             string
	fs                  afero.Fs
}

// Option modifies a Context
type Option func(*Context)

// AllowMissingProfile indicates that Context should still be returned even if a
// profile name is supplied and it does not exist in config.
func AllowMissingProfile() Option {
	return func(ctx *Context) {
		ctx.allowMissingProfile = true
	}
}

// NewFromFlags constructs a new context from flags.
func NewFromFlags(f Flags, opts ...Option) (*Context, error) { //nolint:gocyclo
	p, err := config.GetDefaultPath()
	if err != nil {
		return nil, err
	}

	c := &Context{
		fs:      afero.NewOsFs(),
		cfgPath: p,
	}

	for _, o := range opts {
		o(c)
	}

	src := config.NewFSSource(
		config.WithFS(c.fs),
		config.WithPath(c.cfgPath),
	)
	if err := src.Initialize(); err != nil {
		return nil, err
	}
	conf, err := config.Extract(src)
	if err != nil {
		return nil, err
	}

	c.Cfg = conf
	c.CfgSrc = src

	// If profile identifier is not provided, use the default, or empty if the
	// default cannot be obtained.
	c.Profile = profile.Profile{}
	if f.Profile == "" {
		if name, p, err := c.Cfg.GetDefaultUpboundProfile(); err == nil {
			c.Profile = p
			c.ProfileName = name
		}
	} else {
		p, err := c.Cfg.GetUpboundProfile(f.Profile)
		if err != nil && !c.allowMissingProfile {
			return nil, errors.Errorf(errProfileNotFoundFmt, f.Profile)
		}
		c.Profile = p
		c.ProfileName = f.Profile
	}

	of, err := c.applyOverrides(f, c.ProfileName)
	if err != nil {
		return nil, err
	}

	c.APIEndpoint = of.APIEndpoint
	if c.APIEndpoint == nil {
		u := *of.Domain
		u.Host = apiSubdomain + u.Host
		c.APIEndpoint = &u
	}

	c.AuthEndpoint = of.AuthEndpoint
	if c.AuthEndpoint == nil {
		u := *of.Domain
		u.Host = authSubdomain + u.Host
		c.AuthEndpoint = &u
	}

	c.ProxyEndpoint = of.ProxyEndpoint
	if c.ProxyEndpoint == nil {
		u := *of.Domain
		u.Host = proxySubdomain + u.Host
		u.Path = proxyPath
		c.ProxyEndpoint = &u
	}

	c.RegistryEndpoint = of.RegistryEndpoint
	if c.RegistryEndpoint == nil {
		u := *of.Domain
		u.Host = xpkgSubdomain + u.Host
		c.RegistryEndpoint = &u
	}

	c.Account = of.Account
	c.Domain = of.Domain

	// If account has not already been set, use the profile default.
	if c.Account == "" {
		c.Account = c.Profile.Account
	}

	c.InsecureSkipTLSVerify = of.InsecureSkipTLSVerify

	c.DebugLevel = of.Debug
	switch {
	case of.Debug >= 3:
		c.WrapTransport = func(rt http.RoundTripper) http.RoundTripper {
			return transport.NewDebuggingRoundTripper(rt, transport.DebugCurlCommand, transport.DebugURLTiming, transport.DebugDetailedTiming, transport.DebugResponseHeaders)
		}
	case of.Debug >= 2:
		c.WrapTransport = func(rt http.RoundTripper) http.RoundTripper {
			return transport.NewDebuggingRoundTripper(rt, transport.DebugJustURL, transport.DebugRequestHeaders, transport.DebugResponseStatus, transport.DebugResponseHeaders)
		}
	case of.Debug >= 1:
		c.WrapTransport = func(rt http.RoundTripper) http.RoundTripper {
			return transport.NewDebuggingRoundTripper(rt, transport.DebugURLTiming)
		}
	default:
	}

	c.Kubecfg = clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		clientcmd.NewDefaultClientConfigLoadingRules(),
		&clientcmd.ConfigOverrides{},
	)

	return c, nil
}

// BuildSDKConfig builds an Upbound SDK config suitable for usage with any
// service client.
func (c *Context) BuildSDKConfig() (*up.Config, error) {
	return c.buildSDKConfig(c.APIEndpoint)
}

// BuildSDKAuthConfig builds an Upbound SDK config pointed at the Upbound auth
// endpoint.
func (c *Context) BuildSDKAuthConfig() (*up.Config, error) {
	return c.buildSDKConfig(c.AuthEndpoint)
}

func (c *Context) buildSDKConfig(endpoint *url.URL) (*up.Config, error) {
	cj, err := cookiejar.New(nil)
	if err != nil {
		return nil, err
	}
	if c.Profile.Session != "" {
		cj.SetCookies(c.APIEndpoint, []*http.Cookie{{
			Name:  CookieName,
			Value: c.Profile.Session,
		},
		})
	}
	var tr http.RoundTripper = &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: c.InsecureSkipTLSVerify, //nolint:gosec
		},
	}
	if c.WrapTransport != nil {
		tr = c.WrapTransport(tr)
	}
	client := up.NewClient(func(u *up.HTTPClient) {
		u.BaseURL = endpoint
		u.HTTP = &http.Client{
			Jar:       cj,
			Transport: tr,
		}
		u.UserAgent = UserAgent
	})
	return up.NewConfig(func(conf *up.Config) {
		conf.Client = client
	}), nil
}

type cookieImpersonatingRoundTripper struct {
	session string
	rt      http.RoundTripper
}

func (rt *cookieImpersonatingRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	req = utilnet.CloneRequest(req)
	req.AddCookie(&http.Cookie{
		Name:  CookieName,
		Value: rt.session,
	})
	return rt.rt.RoundTrip(req)
}

// BuildControllerClientConfig builds a REST config suitable for usage with any
// K8s controller-runtime client.
func (c *Context) BuildControllerClientConfig() (*rest.Config, error) {
	var tr http.RoundTripper = &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: c.InsecureSkipTLSVerify, //nolint:gosec
		},
	}

	// mcp-api doesn't support bearer token auth through to spaces APIs, yet.
	// For now, we need to add the SID cookie to every request to authenticate
	// it.
	tr = &cookieImpersonatingRoundTripper{session: c.Profile.Session, rt: tr}

	if c.WrapTransport != nil {
		tr = c.WrapTransport(tr)
	}

	cfg := &rest.Config{
		Host:      c.APIEndpoint.String(),
		APIPath:   controllerClientPath,
		Transport: tr,
		UserAgent: UserAgent,
	}

	if c.Profile.Session != "" {
		cfg.BearerToken = c.Profile.Session
	}
	return cfg, nil
}

// BuildCloudSpaceClientConfig builds a Kubeconfig pointed at an Upbound
// cloud-hosted space. Uses the space name as the current context
func (c *Context) BuildCloudSpaceClientConfig(ctx context.Context, spaceName, organization string) (clientcmd.ClientConfig, error) {
	// describe the space to get its fqdn
	cloudLoader, err := c.BuildControllerClientConfig()
	if err != nil {
		return nil, err
	}

	cloudClient, err := client.New(cloudLoader, client.Options{})
	if err != nil {
		return nil, err
	}

	var space upboundv1alpha1.Space
	err = cloudClient.Get(ctx, types.NamespacedName{Name: spaceName, Namespace: organization}, &space)
	if err != nil {
		return nil, err
	}

	// uniquely identify the space
	contextName := fmt.Sprintf("upbound/%s", spaceName)

	clusters := make(map[string]*clientcmdapi.Cluster)
	clusters[spaceName] = &clientcmdapi.Cluster{
		Server: fmt.Sprintf("https://%s.%s", organization, space.Status.FQDN),
		// todo(redbackthomson): replace once a public CA is configured
		InsecureSkipTLSVerify: true, //nolint:gosec
	}

	contexts := make(map[string]*clientcmdapi.Context)
	contexts[contextName] = &clientcmdapi.Context{
		Cluster: spaceName,
	}

	authInfos := make(map[string]*clientcmdapi.AuthInfo)
	if c.Profile.Session != "" {
		// uniquely identify the profile auth info, prefixed to protect against
		// overriding existing auth info in the customer's kubeconfig
		authInfoName := fmt.Sprintf("upbound/%s", organization)
		authInfos[authInfoName] = &clientcmdapi.AuthInfo{
			Token: c.Profile.Session,
		}
		contexts[contextName].AuthInfo = authInfoName
	}

	return clientcmd.NewDefaultClientConfig(clientcmdapi.Config{
		Kind:           "Config",
		APIVersion:     "v1",
		Clusters:       clusters,
		Contexts:       contexts,
		CurrentContext: contextName,
		AuthInfos:      authInfos,
	}, &clientcmd.ConfigOverrides{}), nil
}

// BuildCloudSpaceRestConfig builds a REST config pointed at an Upbound
// cloud-hosted space suitable for usage with any K8s controller-runtime client.
func (c *Context) BuildCloudSpaceRestConfig(ctx context.Context, spaceName, profileName string) (*rest.Config, error) {
	clientCfg, err := c.BuildCloudSpaceClientConfig(ctx, spaceName, profileName)
	if err != nil {
		return nil, err
	}

	restCfg, err := clientCfg.ClientConfig()
	if err != nil {
		return nil, err
	}

	restCfg.UserAgent = UserAgent

	return restCfg, nil
}

// applyOverrides applies applicable overrides to the given Flags based on the
// pre-existing configs, if there are any.
func (c *Context) applyOverrides(f Flags, profileName string) (Flags, error) {
	// profile doesn't exist, return the supplied flags
	if _, ok := c.Cfg.Upbound.Profiles[profileName]; !ok {
		return f, nil
	}

	of := Flags{}

	baseReader, err := c.Cfg.BaseToJSON(profileName)
	if err != nil {
		return of, err
	}

	overlayBytes, err := json.Marshal(f)
	if err != nil {
		return of, err
	}

	resolver, err := JSON(baseReader, bytes.NewReader(overlayBytes))
	if err != nil {
		return of, err
	}
	parser, err := kong.New(&of, kong.Resolvers(resolver))
	if err != nil {
		return of, err
	}

	if _, err = parser.Parse([]string{}); err != nil {
		return of, err
	}

	return of, nil
}

// MarshalJSON marshals the Flags struct, converting the url.URL to strings.
func (f Flags) MarshalJSON() ([]byte, error) {
	flags := struct {
		Domain                string `json:"domain,omitempty"`
		Profile               string `json:"profile,omitempty"`
		Account               string `json:"account,omitempty"`
		InsecureSkipTLSVerify bool   `json:"insecure_skip_tls_verify,omitempty"`
		Debug                 int    `json:"debug,omitempty"`
		APIEndpoint           string `json:"override_api_endpoint,omitempty"`
		AuthEndpoint          string `json:"override_auth_endpoint,omitempty"`
		ProxyEndpoint         string `json:"override_proxy_endpoint,omitempty"`
		RegistryEndpoint      string `json:"override_registry_endpoint,omitempty"`
	}{
		Domain:                nullableURL(f.Domain),
		Profile:               f.Profile,
		Account:               f.Account,
		InsecureSkipTLSVerify: f.InsecureSkipTLSVerify,
		Debug:                 f.Debug,
		APIEndpoint:           nullableURL(f.APIEndpoint),
		AuthEndpoint:          nullableURL(f.AuthEndpoint),
		ProxyEndpoint:         nullableURL(f.ProxyEndpoint),
		RegistryEndpoint:      nullableURL(f.RegistryEndpoint),
	}
	return json.Marshal(flags)
}

func nullableURL(u *url.URL) string {
	if u == nil {
		return ""
	}
	return u.String()
}
