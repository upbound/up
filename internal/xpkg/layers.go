// Copyright 2022 Upbound Inc
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

package xpkg

import (
	"archive/tar"
	"bytes"
	"io"

	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
	"github.com/pkg/errors"
)

// PackageAddendum creates a mutate.Addendum that contains the package details
// (XRs, XRDs, CRDs, etc) that make up a xpkg in its layer as well as the
// base annotation.
func PackageAddendum(b *bytes.Buffer) (mutate.Addendum, error) {
	// Write on-disk package contents to tarball.
	tarBuf := new(bytes.Buffer)
	tw := tar.NewWriter(tarBuf)

	hdr := &tar.Header{
		Name: StreamFile,
		Mode: int64(StreamFileMode),
		Size: int64(b.Len()),
	}

	if err := writeLayer(tw, hdr, b); err != nil {
		return mutate.Addendum{}, err
	}

	// Build image layer from tarball.
	layer, err := tarball.LayerFromReader(tarBuf)
	if err != nil {
		return mutate.Addendum{}, errors.Wrap(err, errLayerFromTar)
	}

	return mutate.Addendum{
		Layer: layer,
		Annotations: map[string]string{
			AnnotationKey: PackageAnnotation,
		},
	}, nil
}

// ExamplesAddendum creates a mutate.Addendum that contains the examples for the
// xpkg as well as the upbound annotation.
func ExamplesAddendum(b *bytes.Buffer) (mutate.Addendum, error) {
	tarBuf := new(bytes.Buffer)
	tw := tar.NewWriter(tarBuf)

	exHdr := &tar.Header{
		Name: XpkgExamplesFile,
		Mode: int64(StreamFileMode),
		Size: int64(b.Len()),
	}

	if err := writeLayer(tw, exHdr, b); err != nil {
		return mutate.Addendum{}, err
	}

	layer, err := tarball.LayerFromReader(tarBuf)
	if err != nil {
		return mutate.Addendum{}, errors.Wrap(err, errLayerFromTar)
	}

	return mutate.Addendum{
		Layer: layer,
		Annotations: map[string]string{
			AnnotationKey: ExamplesAnnotation,
		},
	}, nil
}

func writeLayer(tw *tar.Writer, hdr *tar.Header, buf io.Reader) error {
	if err := tw.WriteHeader(hdr); err != nil {
		return errors.Wrap(err, errTarFromStream)
	}

	if _, err := io.Copy(tw, buf); err != nil {
		return errors.Wrap(err, errTarFromStream)
	}
	if err := tw.Close(); err != nil {
		return errors.Wrap(err, errTarFromStream)
	}
	return nil
}
