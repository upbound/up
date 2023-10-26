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
	"io"
	"os"
	"testing"

	"github.com/crossplane/crossplane-runtime/pkg/test"
	"github.com/google/go-cmp/cmp"
	"github.com/pterm/pterm"

	"github.com/upbound/up/internal/config"
	"github.com/upbound/up/internal/profile"
	"github.com/upbound/up/internal/upbound"
)

var _ config.Source = &mockConfigSource{}

type mockConfigSource struct {
	cfg *config.Config
}

func (m *mockConfigSource) Initialize() error {
	return nil
}

func (m *mockConfigSource) GetConfig() (*config.Config, error) {
	return m.cfg, nil
}

func (m *mockConfigSource) UpdateConfig(cfg *config.Config) error {
	m.cfg = cfg
	return nil
}

func TestSpaceCmd_Run(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("error getting working directory: %s", err)
	}
	kubeconfig := wd + "/testdata/kubeconfig"

	type args struct {
		ctx *upbound.Context
	}
	type want struct {
		cfg *config.Config
		err error
	}
	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"EmptyConfigDefaultProfile": {
			reason: "Setting the default profile with empty config creates a new default profile.",
			args: args{
				ctx: &upbound.Context{
					Account: "test-account",
					Cfg:     &config.Config{},
				},
			},
			want: want{
				cfg: &config.Config{Upbound: config.Upbound{
					Default: "default",
					Profiles: map[string]profile.Profile{
						"default": {
							Account:     "test-account",
							Type:        "space",
							Kubeconfig:  kubeconfig,
							KubeContext: "default-context",
						},
					},
				}},
			},
		},
		"PopulatedConfigDefaultProfile": {
			reason: "Setting the default profile with populated config updates the default profile.",
			args: args{
				ctx: &upbound.Context{
					Account: "test-account",
					Cfg: &config.Config{Upbound: config.Upbound{
						Default: "default",
						Profiles: map[string]profile.Profile{
							"default": {
								Account:     "test-account",
								Type:        "space",
								Kubeconfig:  "foo",
								KubeContext: "bar",
							},
							"cloud": {
								Account: "test-account",
								Type:    "token",
								ID:      "faketoken",
								Session: "fakesession",
							},
						},
					}},
				},
			},
			want: want{
				cfg: &config.Config{Upbound: config.Upbound{
					Default: "default",
					Profiles: map[string]profile.Profile{
						"default": {
							Account:     "test-account",
							Type:        "space",
							Kubeconfig:  kubeconfig,
							KubeContext: "default-context",
						},
						"cloud": {
							Account: "test-account",
							Type:    "token",
							ID:      "faketoken",
							Session: "fakesession",
						},
					},
				}},
			},
		},
		"CreateProfile": {
			reason: "Passing the name of a nonexistent profile creates that profile.",
			args: args{
				ctx: &upbound.Context{
					ProfileName: "other-profile",
					Account:     "test-account",
					Cfg: &config.Config{Upbound: config.Upbound{
						Default: "default",
						Profiles: map[string]profile.Profile{
							"default": {
								Account:     "test-account",
								Type:        "space",
								Kubeconfig:  "kubeconfig",
								KubeContext: "context",
							},
						},
					}},
				},
			},
			want: want{
				cfg: &config.Config{Upbound: config.Upbound{
					Default: "default",
					Profiles: map[string]profile.Profile{
						"default": {
							Account:     "test-account",
							Type:        "space",
							Kubeconfig:  "kubeconfig",
							KubeContext: "context",
						},
						"other-profile": {
							Account:     "test-account",
							Type:        "space",
							Kubeconfig:  kubeconfig,
							KubeContext: "default-context",
						},
					},
				}},
			},
		},
		"UpdateProfile": {
			reason: "Passing the name of an existent profile updates that profile.",
			args: args{
				ctx: &upbound.Context{
					ProfileName: "other-profile",
					Account:     "test-account",
					Cfg: &config.Config{Upbound: config.Upbound{
						Default: "default",
						Profiles: map[string]profile.Profile{
							"default": {
								Account:     "test-account",
								Type:        "space",
								Kubeconfig:  "kubeconfig",
								KubeContext: "context",
							},
							"other-profile": {
								Account: "test-account",
								Type:    "token",
								ID:      "faketoken",
								Session: "fakesession",
							},
						},
					}},
				},
			},
			want: want{
				cfg: &config.Config{Upbound: config.Upbound{
					Default: "default",
					Profiles: map[string]profile.Profile{
						"default": {
							Account:     "test-account",
							Type:        "space",
							Kubeconfig:  "kubeconfig",
							KubeContext: "context",
						},
						"other-profile": {
							Account:     "test-account",
							Type:        "space",
							Kubeconfig:  kubeconfig,
							KubeContext: "default-context",
						},
					},
				}},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			cmd := &spaceCmd{Kube: upbound.KubeFlags{Kubeconfig: kubeconfig}}
			if diff := cmp.Diff(nil, cmd.AfterApply(nil), test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nspaceCmd.AfterApply(...): -want error, +got error:\n%s", tc.reason, diff)
			}

			cfgSrc := &mockConfigSource{cfg: tc.args.ctx.Cfg}
			tc.args.ctx.CfgSrc = cfgSrc
			p := pterm.DefaultBasicText.WithWriter(io.Discard)
			if diff := cmp.Diff(tc.want.err, cmd.Run(p, tc.args.ctx), test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nspaceCmd.Run(...): -want error, +got error:\n%s", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.cfg, cfgSrc.cfg); diff != "" {
				t.Errorf("\n%s\nspaceCmd.Run(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}
