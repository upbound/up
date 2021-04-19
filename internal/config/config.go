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

	// Profiles contain sets of credentials for communicating with Upbound Cloud.
	Profiles []Profile `json:"profiles,omitempty"`
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

	// An identifier can be one of username, email, or token ID.
	Identifier string `json:"identifier"`

	// Session is a session token used to authenticate to Upbound Cloud.
	Session string `json:"session,omitempty"`

	// Org is the default organization to use when this profile is selected.
	Org string `json:"org,omitempty"`
}

// checkProfile ensures a profile does not violate constraints.
func checkProfile(p Profile) error {
	if p.Type == "" || p.Identifier == "" {
		return errors.New(errInvalidProfile)
	}
	return nil
}

// AddOrUpdateCloudProfile adds or updates a cloud profile to the Config.
func (c *Config) AddOrUpdateCloudProfile(new Profile) error {
	if err := checkProfile(new); err != nil {
		return err
	}
	for i, p := range c.Cloud.Profiles {
		if p.Identifier == new.Identifier {
			c.Cloud.Profiles[i] = new
			return nil
		}
	}
	c.Cloud.Profiles = append(c.Cloud.Profiles, new)
	return nil
}

// GetDefaultCloudProfile gets the default cloud profile or returns an error if
// default is not set or default profile does not exist.
func (c *Config) GetDefaultCloudProfile() (Profile, error) {
	if c.Cloud.Default == "" {
		return Profile{}, errors.New(errNoDefaultSpecified)
	}
	for _, p := range c.Cloud.Profiles {
		if p.Identifier == c.Cloud.Default {
			return p, nil
		}
	}
	return Profile{}, errors.New(errDefaultNotExist)
}

// GetCloudProfile gets a profile with a given identifier. If a profile does not
// exist for the given identifier an error will be returned. Multiple profiles
// should never exist for the same identifier, but in the case that they do, the
// first will be returned.
func (c *Config) GetCloudProfile(id string) (Profile, error) {
	for _, p := range c.Cloud.Profiles {
		if p.Identifier == id {
			return p, nil
		}
	}
	return Profile{}, errors.Errorf(errProfileNotFoundFmt, id)
}

// SetDefaultCloudProfile sets the default profile for communicating with
// Upbound Cloud. Setting a default profile that does not exist will return an
// error.
func (c *Config) SetDefaultCloudProfile(id string) error {
	profileExists := false
	for _, p := range c.Cloud.Profiles {
		if p.Identifier == id {
			c.Cloud.Default = id
			profileExists = true
		}
	}
	if !profileExists {
		return errors.Errorf(errProfileNotFoundFmt, id)
	}
	return nil
}
