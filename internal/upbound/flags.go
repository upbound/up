// Copyright 2023 Upbound Inc
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
	"net/url"

	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// Flags are common flags used by commands that interact with Upbound.
type Flags struct {
	// Optional
	Domain  *url.URL `env:"UP_DOMAIN" default:"https://upbound.io" help:"Root Upbound domain." json:"domain,omitempty"`
	Profile string   `env:"UP_PROFILE" help:"Profile used to execute command." predictor:"profiles" json:"profile,omitempty"`
	Account string   `short:"a" env:"UP_ACCOUNT" help:"Account used to execute command." json:"account,omitempty"`

	// Insecure
	InsecureSkipTLSVerify bool `env:"UP_INSECURE_SKIP_TLS_VERIFY" help:"[INSECURE] Skip verifying TLS certificates." json:"insecureSkipTLSVerify,omitempty"`
	Debug                 int  `short:"d" env:"UP_DEBUG" name:"debug" type:"counter" help:"[INSECURE] Run with debug logging. Repeat to increase verbosity. Output might contain confidential data like tokens." json:"debug,omitempty"`

	// Hidden
	APIEndpoint      *url.URL `env:"OVERRIDE_API_ENDPOINT" hidden:"" name:"override-api-endpoint" help:"Overrides the default API endpoint." json:"apiEndpoint,omitempty"`
	ProxyEndpoint    *url.URL `env:"OVERRIDE_PROXY_ENDPOINT" hidden:"" name:"override-proxy-endpoint" help:"Overrides the default proxy endpoint." json:"proxyEndpoint,omitempty"`
	RegistryEndpoint *url.URL `env:"OVERRIDE_REGISTRY_ENDPOINT" hidden:"" name:"override-registry-endpoint" help:"Overrides the default registry endpoint." json:"registryEndpoint,omitempty"`
}

type KubeFlags struct {
	// Kubeconfig is the kubeconfig file path to read. If empty, it refers to
	// client-go's default kubeconfig location.
	Kubeconfig string `type:"existingfile" help:"Override default kubeconfig path."`
	// Context is the context within Kubeconfig to read. If empty, it refers
	// to the default context.
	Context string `name:"kubecontext" help:"Override default kubeconfig context."`

	// set by AfterApply
	config  *rest.Config
	context string
}

func (f *KubeFlags) AfterApply() error {
	rules := clientcmd.NewDefaultClientConfigLoadingRules()
	rules.ExplicitPath = f.Kubeconfig
	loader := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		rules,
		&clientcmd.ConfigOverrides{CurrentContext: f.Context},
	)

	f.context = f.Context
	if f.context == "" {
		// Get the name of the default context so we can set it explicitly.
		rawConfig, err := loader.RawConfig()
		if err != nil {
			return err
		}
		f.context = rawConfig.CurrentContext
	}

	restConfig, err := loader.ClientConfig()
	if err != nil {
		return err
	}
	f.config = restConfig

	return nil
}

// GetConfig returns the *rest.Config from KubeFlags. Returns nil unless
// AfterApply has been called.
func (f *KubeFlags) GetConfig() *rest.Config {
	return f.config
}

// GetContext returns the kubeconfig context from KubeFlags. Returns empty
// string unless AfterApply has been called. Returns KubeFlags.Context if it's
// defined, otherwise the name of the default context in the config resolved
// from KubeFlags.Kubeconfig.
// NOTE(branden): This ensures that a profile created from this context will
// continue to work with the same cluster if the kubeconfig's default context
// is changed.
func (f *KubeFlags) GetContext() string {
	return f.context
}
