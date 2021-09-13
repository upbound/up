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
	"encoding/json"
	"os"
	"testing"

	"github.com/crossplane/crossplane-runtime/pkg/test"
	"github.com/google/go-cmp/cmp"
	"github.com/spf13/afero"
)

// TODO(hasheddan): a mock afero.Fs could increase test coverage here with
// simulated failed file opens and writes.

func TestGetConfig(t *testing.T) {
	testConf := &Config{
		Upbound: Upbound{
			Default: "test",
		},
	}
	cases := map[string]struct {
		reason    string
		modifiers []FSSourceModifier
		want      *Config
		err       error
	}{
		"SuccessfulEmptyConfig": {
			reason: "An empty file should return an empty config.",
			modifiers: []FSSourceModifier{
				func(f *FSSource) {
					f.fs = afero.NewMemMapFs()
				},
			},
			want: &Config{},
		},
		"SuccessfulAlternateHome": {
			reason: "Setting an alternate home directory should resolve correctly.",
			modifiers: []FSSourceModifier{
				func(f *FSSource) {
					f.fs = afero.NewMemMapFs()
					f.home = func() (string, error) {
						return "/", nil
					}
				},
			},
			want: &Config{},
		},
		"Successful": {
			reason: "Setting an alternate home directory should resolve correctly.",
			modifiers: []FSSourceModifier{
				func(f *FSSource) {
					f.home = func() (string, error) {
						return "/", nil
					}
					fs := afero.NewMemMapFs()
					file, _ := fs.OpenFile("/.up/config.json", os.O_CREATE, 0600)
					defer file.Close()
					b, _ := json.Marshal(testConf)
					_, _ = file.Write(b)
					f.fs = fs
				},
			},
			want: testConf,
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			src, err := NewFSSource(tc.modifiers...)
			if err != nil {
				t.Fatal(err)
			}
			conf, err := src.GetConfig()
			if diff := cmp.Diff(tc.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nGetConfig(...): -want error, +got error:\n%s", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want, conf); diff != "" {
				t.Errorf("\n%s\nGetConfig(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestUpdateConfig(t *testing.T) {
	testConf := &Config{
		Upbound: Upbound{
			Default: "test",
		},
	}
	cases := map[string]struct {
		reason    string
		modifiers []FSSourceModifier
		conf      *Config
		err       error
	}{
		"EmptyConfig": {
			reason: "Updating with empty config should not cause an error.",
			modifiers: []FSSourceModifier{
				func(f *FSSource) {
					f.fs = afero.NewMemMapFs()
				},
			},
		},
		"PopulatedConfig": {
			reason: "Updating with populated config should not cause an error.",
			modifiers: []FSSourceModifier{
				func(f *FSSource) {
					f.fs = afero.NewMemMapFs()
				},
			},
			conf: testConf,
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			src, err := NewFSSource(tc.modifiers...)
			if err != nil {
				t.Fatal(err)
			}
			err = src.UpdateConfig(tc.conf)
			if diff := cmp.Diff(tc.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nUpdateConfig(...): -want error, +got error:\n%s", tc.reason, diff)
			}
		})
	}
}
