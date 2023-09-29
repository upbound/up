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
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"

	"github.com/crossplane/crossplane-runtime/pkg/errors"

	"github.com/upbound/up/internal/profile"
)

// Location of up config file.
const (
	ConfigDir  = ".up"
	ConfigFile = "config.json"
)

const (
	errDefaultNotExist    = "profile specified as default does not exist"
	errNoDefaultSpecified = "no default profile specified"

	errProfileNotFoundFmt = "profile not found with identifier: %s"
	errNoProfilesFound    = "no profiles found"
)

// QuietFlag provides a named boolean type for the QuietFlag.
type QuietFlag bool

// Allowed values for the global format option
type Format string

const (
	Default Format = "default"
	JSON    Format = "json"
	YAML    Format = "yaml"
)

// Config is format for the up configuration file.
type Config struct {
	Upbound Upbound `json:"upbound"`
}

// Extract performs extraction of configuration from the provided source.
func Extract(src Source) (*Config, error) {
	conf, err := src.GetConfig()
	if err != nil {
		return nil, err
	}
	return conf, nil
}

// GetDefaultPath returns the default config path or error.
func GetDefaultPath() (string, error) {
	h, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(h, ConfigDir, ConfigFile), nil
}

// Upbound contains configuration information for Upbound.
type Upbound struct {
	// Default indicates the default profile.
	Default string `json:"default"`

	// Profiles contain sets of credentials for communicating with Upbound. Key
	// is name of the profile.
	Profiles map[string]profile.Profile `json:"profiles,omitempty"`
}

// AddOrUpdateUpboundProfile adds or updates an Upbound profile to the Config.
func (c *Config) AddOrUpdateUpboundProfile(name string, new profile.Profile) error {
	if err := new.Validate(); err != nil {
		return err
	}
	if c.Upbound.Profiles == nil {
		c.Upbound.Profiles = map[string]profile.Profile{}
	}
	c.Upbound.Profiles[name] = new
	return nil
}

// GetDefaultUpboundProfile gets the default Upbound profile or returns an error if
// default is not set or default profile does not exist.
func (c *Config) GetDefaultUpboundProfile() (string, profile.Profile, error) {
	if c.Upbound.Default == "" {
		return "", profile.Profile{}, errors.New(errNoDefaultSpecified)
	}
	p, ok := c.Upbound.Profiles[c.Upbound.Default]
	if !ok {
		return "", profile.Profile{}, errors.New(errDefaultNotExist)
	}
	return c.Upbound.Default, p, nil
}

// GetUpboundProfile gets a profile with a given identifier. If a profile does not
// exist for the given identifier an error will be returned. Multiple profiles
// should never exist for the same identifier, but in the case that they do, the
// first will be returned.
func (c *Config) GetUpboundProfile(name string) (profile.Profile, error) {
	p, ok := c.Upbound.Profiles[name]
	if !ok {
		return profile.Profile{}, errors.Errorf(errProfileNotFoundFmt, name)
	}
	return p, nil
}

// GetUpboundProfiles returns the list of existing profiles. If no profiles
// exist, then an error will be returned.
func (c *Config) GetUpboundProfiles() (map[string]profile.Profile, error) {
	if c.Upbound.Profiles == nil {
		return nil, errors.New(errNoProfilesFound)
	}

	return c.Upbound.Profiles, nil
}

// SetDefaultUpboundProfile sets the default profile for communicating with
// Upbound. Setting a default profile that does not exist will return an
// error.
func (c *Config) SetDefaultUpboundProfile(name string) error {
	if _, ok := c.Upbound.Profiles[name]; !ok {
		return errors.Errorf(errProfileNotFoundFmt, name)
	}
	c.Upbound.Default = name
	return nil
}

// GetBaseConfig returns the persisted base configuration associated with the
// provided Profile. If the supplied name does not match an existing Profile
// an error is returned.
func (c *Config) GetBaseConfig(name string) (map[string]string, error) {
	profile, ok := c.Upbound.Profiles[name]
	if !ok {
		return nil, errors.Errorf(errProfileNotFoundFmt, name)
	}
	return profile.BaseConfig, nil
}

// AddToBaseConfig adds the supplied key, value pair to the base config map of
// the profile that corresponds to the given name. If the supplied name does
// not match an existing Profile an error is returned. If the overrides map
// does not currently exist on the corresponding profile, a map is initialized.
func (c *Config) AddToBaseConfig(name, key, value string) error {
	profile, ok := c.Upbound.Profiles[name]
	if !ok {
		return errors.Errorf(errProfileNotFoundFmt, name)
	}

	if profile.BaseConfig == nil {
		profile.BaseConfig = make(map[string]string)
	}

	profile.BaseConfig[key] = value
	c.Upbound.Profiles[name] = profile
	return nil
}

// RemoveFromBaseConfig removes the supplied key from the base config map of
// the Profile that corresponds to the given name. If the supplied name does
// not match an existing Profile an error is returned. If the base config map
// does not currently exist on the corresponding profile, a no-op occurs.
func (c *Config) RemoveFromBaseConfig(name, key string) error {
	profile, ok := c.Upbound.Profiles[name]
	if !ok {
		return errors.Errorf(errProfileNotFoundFmt, name)
	}

	if profile.BaseConfig == nil {
		return nil
	}

	delete(profile.BaseConfig, key)
	c.Upbound.Profiles[name] = profile
	return nil
}

// BaseToJSON converts the base config of the given Profile to JSON. If the
// config couldn't be converted or if the supplied name does not correspond
// to an existing Profile, an error is returned.
func (c *Config) BaseToJSON(name string) (io.Reader, error) {
	profile, err := c.GetBaseConfig(name)
	if err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(profile); err != nil {
		return nil, err
	}

	return &buf, nil
}
