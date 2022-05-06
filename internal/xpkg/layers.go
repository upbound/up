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

// Addendum creates a mutate.Addendum that contains the layer contents for the
// xpkg as well as the specified annotation.
func Addendum(r io.Reader, fileName, annotation string, fileSize int64) (mutate.Addendum, error) {
	tarBuf := new(bytes.Buffer)
	tw := tar.NewWriter(tarBuf)

	exHdr := &tar.Header{
		Name: fileName,
		Mode: int64(StreamFileMode),
		Size: fileSize,
	}

	if err := writeLayer(tw, exHdr, r); err != nil {
		return mutate.Addendum{}, err
	}

	layer, err := tarball.LayerFromReader(tarBuf)
	if err != nil {
		return mutate.Addendum{}, errors.Wrap(err, errLayerFromTar)
	}

	return mutate.Addendum{
		Layer: layer,
		Annotations: map[string]string{
			AnnotationKey: annotation,
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
