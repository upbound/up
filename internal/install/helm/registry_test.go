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

package helm

import (
	"net/url"
	"testing"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/test"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/random"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/types"
	"github.com/spf13/afero"
)

func newFetchFn(img v1.Image, err error) fetchFn {
	return func(ref name.Reference, options ...remote.Option) (v1.Image, error) {
		return img, err
	}
}

func TestRegistryPullerRun(t *testing.T) {
	errBoom := errors.New("boom")
	version := "1.0.0"
	chart := "enterprise"
	u, _ := url.Parse("registry.upbound.io/enterprise")
	img, _ := random.Image(1, 1)
	badImg, _ := random.Image(1, 2)
	cases := map[string]struct {
		reason    string
		puller    *registryPuller
		chartName string
		err       error
	}{
		"ErrorParseName": {
			reason: "If image name reference is not valid we should return an error.",
			puller: &registryPuller{
				version: version,
				repoURL: u,
			},
			chartName: "~?",
			err:       errors.Wrap(errors.New("could not parse reference: registry.upbound.io/enterprise/~?:1.0.0"), errImageReference),
		},
		"ErrorFetch": {
			reason: "If we fail to fetch image we should return an error.",
			puller: &registryPuller{
				fetch:   newFetchFn(nil, errBoom),
				version: version,
				repoURL: u,
			},
			chartName: chart,
			err:       errors.Wrap(errBoom, errGetImage),
		},
		"ErrorWrongNumberOfLayers": {
			reason: "If we fetch image but it has the wrong number of layers we should return an error.",
			puller: &registryPuller{
				fetch:   newFetchFn(badImg, nil),
				version: version,
				repoURL: u,
			},
			chartName: chart,
			err:       errors.New(errNotSingleLayer),
		},
		"ErrorWrongMediaType": {
			reason: "If we fetch image but it has a single layer with an unacceptable media type we should return an error.",
			puller: &registryPuller{
				fetch:             newFetchFn(img, nil),
				acceptedMediaType: "obscurity",
				version:           version,
				repoURL:           u,
			},
			chartName: chart,
			err:       errors.Errorf(errLayerMediaTypeFmt, string(types.DockerLayer), "obscurity"),
		},
		"Successful": {
			reason: "If image is able to be fetched, content is a valid media type, and we successfully write to filesystem no error should be returned.",
			puller: &registryPuller{
				fs:                afero.NewMemMapFs(),
				fetch:             newFetchFn(img, nil),
				acceptedMediaType: string(types.DockerLayer),
				version:           version,
				repoURL:           u,
			},
			chartName: chart,
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := tc.puller.Run(tc.chartName)
			if diff := cmp.Diff(tc.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nRun(...): -want error, +got error:\n%s", tc.reason, diff)
			}
		})
	}
}
