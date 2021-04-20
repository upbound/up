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
	// Cloud. Key is one of username, email, or token ID.
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
	// Type is the type of the profile.
	Type ProfileType `json:"type"`

	// Session is a session token used to authenticate to Upbound Cloud.
	Session string `json:"session,omitempty"`

	// Org is the default organization to use when this profile is selected.
	Org string `json:"org,omitempty"`
}

// checkProfile ensures a profile does not violate constraints.
func checkProfile(p Profile) error {
	if p.Type == "" {
		return errors.New(errInvalidProfile)
	}
	return nil
}

// AddOrUpdateCloudProfile adds or updates a cloud profile to the Config.
func (c *Config) AddOrUpdateCloudProfile(id string, new Profile) error {
	if err := checkProfile(new); err != nil {
		return err
	}
	if c.Cloud.Profiles == nil {
		c.Cloud.Profiles = map[string]Profile{}
	}
	c.Cloud.Profiles[id] = new
	return nil
}

// GetDefaultCloudProfile gets the default cloud profile or returns an error if
// default is not set or default profile does not exist.
func (c *Config) GetDefaultCloudProfile() (Profile, error) {
	if c.Cloud.Default == "" {
		return Profile{}, errors.New(errNoDefaultSpecified)
	}
	p, ok := c.Cloud.Profiles[c.Cloud.Default]
	if !ok {
		return Profile{}, errors.New(errDefaultNotExist)
	}
	return p, nil
}

// GetCloudProfile gets a profile with a given identifier. If a profile does not
// exist for the given identifier an error will be returned. Multiple profiles
// should never exist for the same identifier, but in the case that they do, the
// first will be returned.
func (c *Config) GetCloudProfile(id string) (Profile, error) {
	p, ok := c.Cloud.Profiles[id]
	if !ok {
		return Profile{}, errors.Errorf(errProfileNotFoundFmt, id)
	}
	return p, nil
}

// SetDefaultCloudProfile sets the default profile for communicating with
// Upbound Cloud. Setting a default profile that does not exist will return an
// error.
func (c *Config) SetDefaultCloudProfile(id string) error {
	if _, ok := c.Cloud.Profiles[id]; !ok {
		return errors.Errorf(errProfileNotFoundFmt, id)
	}
	c.Cloud.Default = id
	return nil
}
