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

package config

import (
	"github.com/pkg/errors"
)

// Location of up config file.
const (
	ConfigDir  = ".up"
	ConfigFile = "config.json"
)

const (
	errDefaultNotExist    = "profile specified as default does not exist"
	errNoDefaultSpecified = "no default profile specified"
	errInvalidProfile     = "profile is not valid"

	errProfileNotFoundFmt = "profile not found with identifier: %s"
)

// Config is format for the up configuration file.
type Config struct {
	Cloud Cloud `json:"cloud"`
}

// Cloud contains configuration information for Upbound Cloud.
type Cloud struct {
	// Default indicates the default profile.
	Default string `json:"default"`

	// Profiles contain sets of credentials for communicating with Upbound
	// Cloud. Key is name of the profile.
	Profiles map[string]Profile `json:"profiles,omitempty"`
}

// ProfileType is a type of Upbound Cloud profile.
type ProfileType string

// Types of profiles.
const (
	UserProfileType  ProfileType = "user"
	TokenProfileType ProfileType = "token"
)

// A Profile is a set of credentials
type Profile struct {
	// ID is either a username, email, or token.
	ID string `json:"id"`

	// Type is the type of the profile.
	Type ProfileType `json:"type"`

	// Session is a session token used to authenticate to Upbound Cloud.
	Session string `json:"session,omitempty"`

	// Account is the default account to use when this profile is selected.
	Account string `json:"account,omitempty"`
}

// checkProfile ensures a profile does not violate constraints.
func checkProfile(p Profile) error {
	if p.ID == "" || p.Type == "" {
		return errors.New(errInvalidProfile)
	}
	return nil
}

// AddOrUpdateCloudProfile adds or updates a cloud profile to the Config.
func (c *Config) AddOrUpdateCloudProfile(name string, new Profile) error {
	if err := checkProfile(new); err != nil {
		return err
	}
	if c.Cloud.Profiles == nil {
		c.Cloud.Profiles = map[string]Profile{}
	}
	c.Cloud.Profiles[name] = new
	return nil
}

// GetDefaultCloudProfile gets the default cloud profile or returns an error if
// default is not set or default profile does not exist.
func (c *Config) GetDefaultCloudProfile() (string, Profile, error) {
	if c.Cloud.Default == "" {
		return "", Profile{}, errors.New(errNoDefaultSpecified)
	}
	p, ok := c.Cloud.Profiles[c.Cloud.Default]
	if !ok {
		return "", Profile{}, errors.New(errDefaultNotExist)
	}
	return c.Cloud.Default, p, nil
}

// GetCloudProfile gets a profile with a given identifier. If a profile does not
// exist for the given identifier an error will be returned. Multiple profiles
// should never exist for the same identifier, but in the case that they do, the
// first will be returned.
func (c *Config) GetCloudProfile(name string) (Profile, error) {
	p, ok := c.Cloud.Profiles[name]
	if !ok {
		return Profile{}, errors.Errorf(errProfileNotFoundFmt, name)
	}
	return p, nil
}

// SetDefaultCloudProfile sets the default profile for communicating with
// Upbound Cloud. Setting a default profile that does not exist will return an
// error.
func (c *Config) SetDefaultCloudProfile(name string) error {
	if _, ok := c.Cloud.Profiles[name]; !ok {
		return errors.Errorf(errProfileNotFoundFmt, name)
	}
	c.Cloud.Default = name
	return nil
}
