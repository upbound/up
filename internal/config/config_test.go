package config

import (
	"testing"

	"github.com/crossplane/crossplane-runtime/pkg/test"
	"github.com/google/go-cmp/cmp"
	"github.com/pkg/errors"
)

func TestAddOrUpdateCloudProfile(t *testing.T) {
	name := "cool-profile"
	profOne := Profile{
		ID:   "cool-user",
		Type: UserProfileType,
		Org:  "cool-org",
	}
	profTwo := Profile{
		ID:   "cool-user",
		Type: UserProfileType,
		Org:  "other-org",
	}

	cases := map[string]struct {
		reason string
		name   string
		cfg    *Config
		add    Profile
		want   *Config
		err    error
	}{
		"AddNewProfile": {
			reason: "Adding a new profile to an empty Config should not cause an error.",
			name:   name,
			cfg:    &Config{},
			add:    profOne,
			want: &Config{
				Cloud: Cloud{
					Profiles: map[string]Profile{name: profOne},
				},
			},
		},
		"UpdateExistingProfile": {
			reason: "Updating an existing profile in the Config should not cause an error.",
			name:   name,
			cfg: &Config{
				Cloud: Cloud{
					Profiles: map[string]Profile{name: profOne},
				},
			},
			add: profTwo,
			want: &Config{
				Cloud: Cloud{
					Profiles: map[string]Profile{name: profTwo},
				},
			},
		},
		"Invalid": {
			reason: "Adding an invalid profile should cause an error.",
			name:   name,
			cfg:    &Config{},
			add:    Profile{},
			want:   &Config{},
			err:    errors.New(errInvalidProfile),
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			err := tc.cfg.AddOrUpdateCloudProfile(tc.name, tc.add)
			if diff := cmp.Diff(tc.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nAddOrUpdateCloudProfile(...): -want error, +got error:\n%s", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want, tc.cfg); diff != "" {
				t.Errorf("\n%s\nAddOrUpdateCloudProfile(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestGetDefaultCloudProfile(t *testing.T) {
	name := "cool-profile"
	profOne := Profile{
		ID:   "cool-user",
		Type: UserProfileType,
		Org:  "cool-org",
	}

	cases := map[string]struct {
		reason string
		name   string
		cfg    *Config
		want   Profile
		err    error
	}{
		"ErrorNoDefault": {
			reason: "If no default defined an error should be returned.",
			cfg:    &Config{},
			want:   Profile{},
			err:    errors.New(errNoDefaultSpecified),
		},
		"ErrorDefaultNotExist": {
			reason: "If defined default does not exist an error should be returned.",
			cfg: &Config{
				Cloud: Cloud{
					Default: "test",
				},
			},
			want: Profile{},
			err:  errors.New(errDefaultNotExist),
		},
		"Successful": {
			reason: "If defined default exists it should be returned.",
			name:   name,
			cfg: &Config{
				Cloud: Cloud{
					Default:  "cool-profile",
					Profiles: map[string]Profile{name: profOne},
				},
			},
			want: profOne,
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			name, prof, err := tc.cfg.GetDefaultCloudProfile()
			if diff := cmp.Diff(tc.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nGetDefaultCloudProfile(...): -want error, +got error:\n%s", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.name, name); diff != "" {
				t.Errorf("\n%s\nGetDefaultCloudProfile(...): -want, +got:\n%s", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want, prof); diff != "" {
				t.Errorf("\n%s\nGetDefaultCloudProfile(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestGetCloudProfile(t *testing.T) {
	name := "cool-profile"
	profOne := Profile{
		ID:   "cool-user",
		Type: UserProfileType,
		Org:  "cool-org",
	}

	cases := map[string]struct {
		reason string
		name   string
		cfg    *Config
		want   Profile
		err    error
	}{
		"ErrorProfileNotExist": {
			reason: "If profile does not exist an error should be returned.",
			name:   name,
			cfg:    &Config{},
			want:   Profile{},
			err:    errors.Errorf(errProfileNotFoundFmt, "cool-profile"),
		},
		"Successful": {
			reason: "If profile exists it should be returned.",
			name:   "cool-profile",
			cfg: &Config{
				Cloud: Cloud{
					Profiles: map[string]Profile{name: profOne},
				},
			},
			want: profOne,
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			prof, err := tc.cfg.GetCloudProfile(tc.name)
			if diff := cmp.Diff(tc.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nGetCloudProfile(...): -want error, +got error:\n%s", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want, prof); diff != "" {
				t.Errorf("\n%s\nGetCloudProfile(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestSetDefaultCloudProfile(t *testing.T) {
	name := "cool-user"
	profOne := Profile{
		Type: UserProfileType,
		Org:  "cool-org",
	}

	cases := map[string]struct {
		reason string
		name   string
		cfg    *Config
		err    error
	}{
		"ErrorProfileNotExist": {
			reason: "If profile does not exist an error should be returned.",
			name:   name,
			cfg:    &Config{},
			err:    errors.Errorf(errProfileNotFoundFmt, "cool-user"),
		},
		"Successful": {
			reason: "If profile exists it should be set as default.",
			name:   "cool-user",
			cfg: &Config{
				Cloud: Cloud{
					Profiles: map[string]Profile{name: profOne},
				},
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			err := tc.cfg.SetDefaultCloudProfile(tc.name)
			if diff := cmp.Diff(tc.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nGetCloudProfile(...): -want error, +got error:\n%s", tc.reason, diff)
			}
		})
	}
}
