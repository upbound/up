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

package cache

import (
	"testing"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/test"
	"github.com/google/go-cmp/cmp"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/spf13/afero"
)

const (
	testConfigPkgYaml = "testdata/config_package.yaml"
)

func TestFlush(t *testing.T) {
	fs := afero.NewMemMapFs()

	cache, _ := NewLocal(
		WithFS(fs),
		WithRoot("/cache"),
		rootIsHome,
	)

	type args struct {
		img v1.Image
	}

	type want struct {
		metaCount   int
		crdCount    int
		xrdCount    int
		newEntryErr error
		flushErr    error
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"ProviderSuccess": {
			reason: "Should produce the expected number of definitions from test provider package.",
			args: args{
				img: newPackageImage(testProviderPkgYaml),
			},
			want: want{
				metaCount: 1,
				crdCount:  2,
				xrdCount:  0,
			},
		},
		"ConfigurationSuccess": {
			reason: "Should produce the expected number of definitions from test configuration package.",
			args: args{
				img: newPackageImage(testConfigPkgYaml),
			},
			want: want{
				metaCount: 1,
				crdCount:  0,
				xrdCount:  2,
			},
		},
		"ErrFailedToParsePackageYaml": {
			reason: "Should error if we aren't able to parse the package.yaml in the given package.",
			args: args{
				img: empty.Image,
			},
			want: want{
				newEntryErr: errors.Wrap(errors.New("open package.yaml: no such file or directory"), errOpenPackageStream),
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			e, err := cache.NewEntry(tc.args.img)

			if diff := cmp.Diff(tc.want.newEntryErr, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nFlush(...): -want err, +got err:\n%s", tc.reason, diff)
			}

			if e != nil {
				mc, cc, xc, err := e.flush()

				if diff := cmp.Diff(tc.want.metaCount, mc); diff != "" {
					t.Errorf("\n%s\nFlush(...): -want err, +got err:\n%s", tc.reason, diff)
				}

				if diff := cmp.Diff(tc.want.crdCount, cc); diff != "" {
					t.Errorf("\n%s\nFlush(...): -want err, +got err:\n%s", tc.reason, diff)
				}

				if diff := cmp.Diff(tc.want.xrdCount, xc); diff != "" {
					t.Errorf("\n%s\nFlush(...): -want err, +got err:\n%s", tc.reason, diff)
				}

				if diff := cmp.Diff(tc.want.flushErr, err, test.EquateErrors()); diff != "" {
					t.Errorf("\n%s\nFlush(...): -want err, +got err:\n%s", tc.reason, diff)
				}
			}
		})
	}
}
