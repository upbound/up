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

package profile

import (
	"encoding/json"
	"errors"
	"fmt"

	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// Type is a type of Upbound profile.
type Type string

const (
	// Types of profiles.
	User  Type = "user"
	Token Type = "token"
	Space Type = "space"

	DefaultName = "default"

	errInvalidProfile = "profile is not valid"
)

// A Profile is a set of credentials
type Profile struct {
	// ID is either a username, email, or token.
	ID string `json:"id,omitempty"`

	// Type is the type of the profile.
	Type Type `json:"type"`

	// Session is a session token used to authenticate to Upbound.
	Session string `json:"session,omitempty"`

	// Account is the default account to use when this profile is selected.
	Account string `json:"account,omitempty"`

	// Kubeconfig is the kubeconfig file path that GetKubeClientConfig() will
	// read. If empty, it refers to client-go's default kubeconfig location.
	Kubeconfig string `json:"kubeconfig,omitempty"`

	// KubeContext is the context within Kubeconfig that GetKubeClientConfig()
	// will read. If empty, it refers to the default context.
	KubeContext string `json:"kube_context,omitempty"`

	// BaseConfig represent persisted settings for this profile.
	// For example:
	// * flags
	// * environment variables
	BaseConfig map[string]string `json:"base,omitempty"`
}

// Validate returns an error if the profile is invalid.
func (p Profile) Validate() error {
	if (!p.IsSpace() && p.ID == "") || p.Type == "" {
		return errors.New(errInvalidProfile)
	}
	return nil
}

func (p Profile) IsSpace() bool {
	return p.Type == Space
}

// GetKubeClientConfig returns a *rest.Config loaded from p.Kubeconfig and
// p.KubeContext. It returns an error if p.IsSpace() is false.
func (p Profile) GetKubeClientConfig() (*rest.Config, string, error) {
	if !p.IsSpace() {
		return nil, "", fmt.Errorf("kube client not supported for profile type %q", p.Type)
	}
	rules := clientcmd.NewDefaultClientConfigLoadingRules()
	rules.ExplicitPath = p.Kubeconfig
	loader := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		rules,
		&clientcmd.ConfigOverrides{CurrentContext: p.KubeContext},
	)

	cfg, err := loader.ClientConfig()
	if err != nil {
		return nil, "", err
	}

	ns, _, err := loader.Namespace()
	if err != nil {
		return nil, "", err
	}

	return cfg, ns, nil
}

// Redacted embeds a Upbound Profile for the sole purpose of redacting
// sensitive information.
type Redacted struct {
	Profile
}

// MarshalJSON overrides the session field with `REDACTED` so as not to leak
// sensitive information. We're using an explicit copy here instead of updating
// the underlying Profile struct so as to not modifying the internal state of
// the struct by accident.
func (p Redacted) MarshalJSON() ([]byte, error) {
	type profile Redacted
	pc := profile(p)
	// Space profiles don't have session tokens.
	if !p.IsSpace() {
		s := "NONE"
		if pc.Session != "" {
			s = "REDACTED"
		}
		pc.Session = s
	}
	return json.Marshal(&pc)
}
