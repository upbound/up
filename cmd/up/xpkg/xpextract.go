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

package xpkg

import (
	"archive/tar"
	"compress/gzip"
	"io"
	"path/filepath"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/pkg/errors"
	"github.com/spf13/afero"

	"github.com/upbound/up/internal/xpkg"
)

const (
	errInvalidTag              = "package tag is not a valid reference"
	errFetchPackage            = "failed to fetch package from remote"
	errGetManifest             = "failed to get package image manifest from remote"
	errFetchLayer              = "failed to fetch annotated base layer from remote"
	errGetUncompressed         = "failed to get uncompressed contents from layer"
	errMultipleAnnotatedLayers = "package is invalid due to multiple annotated base layers"
	errOpenPackageStream       = "failed to open package stream file"
	errCreateOutputFile        = "failed to create output file"
	errCreateGzipWriter        = "failed to create gzip writer"
	errExtractPackageContents  = "failed to extract package contents"
)

const (
	layerAnnotation     = "io.crossplane.xpkg"
	baseAnnotationValue = "base"
	cacheContentExt     = ".gz"
)

// AfterApply constructs and binds Upbound-specific context to any subcommands
// that have Run() methods that receive it.
func (c *xpExtractCmd) AfterApply() error {
	c.fs = afero.NewOsFs()
	return nil
}

// xpExtractCmd extracts package contents into a Crossplane cache compatible
// format.
type xpExtractCmd struct {
	fs afero.Fs

	Tag    string `arg:"" help:"Tag of the package to extract. Must be a valid OCI image tag."`
	Output string `short:"o" help:"Package output file path. Extension must be .gz or will be replaced." default:"out.gz"`
}

// Run runs the xp extract cmd.
func (c *xpExtractCmd) Run() error { //nolint:gocyclo
	// NOTE(hasheddan): most of the logic in this method is from the machinery
	// used in Crossplane's package cache and should be updated to use shared
	// libraries if moved to crossplane-runtime.
	tag, err := name.NewTag(c.Tag, name.WithDefaultRegistry(upboundRegistry))
	if err != nil {
		return errors.Wrap(err, errInvalidTag)
	}

	// Fetch package.
	img, err := remote.Image(tag)
	if err != nil {
		return errors.Wrap(err, errFetchPackage)
	}

	// Get image manifest.
	manifest, err := img.Manifest()
	if err != nil {
		return errors.Wrap(err, errGetManifest)
	}

	// Determine if the image is using annotated layers.
	var tarc io.ReadCloser
	foundAnnotated := false
	for _, l := range manifest.Layers {
		if a, ok := l.Annotations[layerAnnotation]; !ok || a != baseAnnotationValue {
			continue
		}
		// NOTE(hasheddan): the xpkg specification dictates that only one layer
		// descriptor may be annotated as xpkg base. Since iterating through all
		// descriptors is relatively inexpensive, we opt to do so in order to
		// verify that we aren't just using the first layer annotated as xpkg
		// base.
		if foundAnnotated {
			return errors.New(errMultipleAnnotatedLayers)
		}
		foundAnnotated = true
		layer, err := img.LayerByDigest(l.Digest)
		if err != nil {
			return errors.Wrap(err, errFetchLayer)
		}
		tarc, err = layer.Uncompressed()
		if err != nil {
			return errors.Wrap(err, errGetUncompressed)
		}
	}

	// If we still don't have content then we need to flatten image filesystem.
	if !foundAnnotated {
		tarc = mutate.Extract(img)
	}

	// The ReadCloser is an uncompressed tarball, either consisting of annotated
	// layer contents or flattened filesystem content. Either way, we only want
	// the package YAML stream.
	t := tar.NewReader(tarc)
	var size int64
	for {
		h, err := t.Next()
		if err != nil {
			return errors.Wrap(err, errOpenPackageStream)
		}
		if h.Name == xpkg.StreamFile {
			size = h.Size
			break
		}
	}

	out := xpkg.ReplaceExt(filepath.Clean(c.Output), cacheContentExt)
	cf, err := c.fs.Create(out)
	if err != nil {
		return errors.Wrap(err, errCreateOutputFile)
	}
	// NOTE(hasheddan): we don't check error on deferred file close as Close()
	// is explicitly called in the happy path.
	defer cf.Close() //nolint:errcheck
	w, err := gzip.NewWriterLevel(cf, gzip.BestSpeed)
	if err != nil {
		return errors.Wrap(err, errCreateGzipWriter)
	}
	if _, err = io.CopyN(w, t, size); err != nil {
		return errors.Wrap(err, errExtractPackageContents)
	}
	// NOTE(hasheddan): gzip writer must be closed to ensure all data is flushed
	// to file.
	if err := w.Close(); err != nil {
		return errors.Wrap(err, errExtractPackageContents)
	}
	return cf.Close()
}
