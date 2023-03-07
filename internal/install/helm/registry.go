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
	"fmt"
	"net/url"
	"path/filepath"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/spf13/afero"
)

const (
	// HelmChartConfigMediaType is the reserved media type for the Helm chart
	// manifest config.
	HelmChartConfigMediaType = "application/vnd.cncf.helm.config.v1+json"

	// HelmChartContentLayerMediaType is the reserved media type for Helm chart
	// package content.
	HelmChartContentLayerMediaType = "application/tar+gzip"
)

const (
	errImageReference    = "failed to parse helm chart repository and name into a valid OCI image reference"
	errGetImage          = "failed to get OCI image"
	errGetImageLayers    = "failed to get OCI image layers"
	errNotSingleLayer    = "OCI image does not have a single layer"
	errLayerMediaTypeFmt = "OCI image layer has media type %s and %s is required"
	errReadCompressed    = "failed to read compressed chart contents"
)

type fetchFn func(ref name.Reference, options ...remote.Option) (v1.Image, error)

var _ helmPuller = &registryPuller{}

type registryPuller struct {
	fs                afero.Fs
	fetch             fetchFn
	acceptedMediaType string

	cacheDir   string
	version    string
	repoURL    *url.URL
	remoteOpts []remote.Option
}

type registryPullerOpt func(*registryPuller)

func withRemoteOpts(opts ...remote.Option) registryPullerOpt {
	return func(r *registryPuller) {
		r.remoteOpts = opts
	}
}

func withRepoURL(u *url.URL) registryPullerOpt {
	return func(r *registryPuller) {
		r.repoURL = u
	}
}

func newRegistryPuller(opts ...registryPullerOpt) *registryPuller {
	r := &registryPuller{
		fs:                afero.NewOsFs(),
		fetch:             remote.Image,
		acceptedMediaType: HelmChartContentLayerMediaType,
	}
	for _, o := range opts {
		o(r)
	}
	return r
}

func (p *registryPuller) Run(chartName string) (string, error) {
	// NOTE(hasheddan): we append v to tag as the helm manager will strip it
	// from version before running the pull client.
	ref, err := name.ParseReference(fmt.Sprintf("%s/%s:v%s", p.repoURL.String(), chartName, p.version))
	if err != nil {
		return "", errors.Wrap(err, errImageReference)
	}
	img, err := p.fetch(ref, p.remoteOpts...)
	if err != nil {
		return "", errors.Wrap(err, errGetImage)
	}
	ls, err := img.Layers()
	if err != nil {
		return "", errors.Wrap(err, errGetImageLayers)
	}
	if len(ls) != 1 {
		return "", errors.New(errNotSingleLayer)
	}
	chart := ls[0]
	mt, err := chart.MediaType()
	if err != nil {
		return "", err
	}
	if string(mt) != p.acceptedMediaType {
		return "", errors.Errorf(errLayerMediaTypeFmt, string(mt), p.acceptedMediaType)
	}
	read, err := chart.Compressed()
	if err != nil {
		return "", errors.Wrap(err, errReadCompressed)
	}
	defer read.Close() //nolint:errcheck
	fileName := filepath.Join(p.cacheDir, fmt.Sprintf("%s-%s.tgz", chartName, p.version))

	// TODO(hasheddan): the native helm pull client will build up a string
	// containing information about operations that took place while acquiring
	// the chart. We currently do not use that information in the Helm manager,
	// so we return an empty string in all cases in the registry puller. We
	// should evaluate in the future if exposing this information is relevant to
	// users.
	return "", afero.WriteReader(p.fs, fileName, read)
}

func (p *registryPuller) SetDestDir(dir string) {
	p.cacheDir = dir
}

func (p *registryPuller) SetVersion(version string) {
	p.version = version
}
