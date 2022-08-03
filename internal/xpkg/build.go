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
	"bytes"
	"context"
	"io"
	"os"
	"strings"

	pkgmetav1 "github.com/crossplane/crossplane/apis/pkg/meta/v1"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/crossplane/crossplane-runtime/pkg/parser"

	"github.com/upbound/up/internal/xpkg/parser/examples"
	"github.com/upbound/up/internal/xpkg/parser/linter"
)

const (
	errParserPackage = "failed to parse package"
	errParserExample = "failed to parse examples"
	errLintPackage   = "failed to lint package"
	errInitBackend   = "failed to initialize package parsing backend"
	errTarFromStream = "failed to build tarball from stream"
	errLayerFromTar  = "failed to convert tarball to image layer"
	errDigestInvalid = "failed to get digest from image layer"
	errBuildImage    = "failed to build image from layers"
	errConfigFile    = "failed to get config file from image"
	errMutateConfig  = "failed to mutate config for image"
)

// annotatedTeeReadCloser is a copy of io.TeeReader that implements
// parser.AnnotatedReadCloser. It returns a Reader that writes to w what it
// reads from r. All reads from r performed through it are matched with
// corresponding writes to w. There is no internal buffering - the write must
// complete before the read completes. Any error encountered while writing is
// reported as a read error. If the underlying reader is a
// parser.AnnotatedReadCloser the tee reader will invoke its Annotate function.
// Otherwise it will return nil. Closing is always a no-op.
func annotatedTeeReadCloser(r io.Reader, w io.Writer) *teeReader {
	return &teeReader{r, w}
}

type teeReader struct {
	r io.Reader
	w io.Writer
}

func (t *teeReader) Read(p []byte) (n int, err error) {
	n, err = t.r.Read(p)
	if n > 0 {
		if n, err := t.w.Write(p[:n]); err != nil {
			return n, err
		}
	}
	return
}

func (t *teeReader) Close() error {
	return nil
}

func (t *teeReader) Annotate() any {
	anno, ok := t.r.(parser.AnnotatedReadCloser)
	if !ok {
		return nil
	}
	return anno.Annotate()
}

// Builder defines an xpkg Builder.
type Builder struct {
	pb parser.Backend
	eb parser.Backend

	pp parser.Parser
	ep *examples.Parser
}

// New returns a new Builder.
func New(pkg, ex parser.Backend, pp parser.Parser, ep *examples.Parser) *Builder {
	return &Builder{
		pb: pkg,
		eb: ex,
		pp: pp,
		ep: ep,
	}
}

type buildOpts struct {
	base v1.Image
}

// A BuildOpt modifies how a package is built.
type BuildOpt func(*buildOpts)

// WithController sets the controller image that should serve as the base for
// the package.
func WithController(img v1.Image) BuildOpt {
	return func(o *buildOpts) {
		o.base = img
	}
}

// Build compiles a Crossplane package from an on-disk package.
func (b *Builder) Build(ctx context.Context, opts ...BuildOpt) (v1.Image, runtime.Object, error) { // nolint:gocyclo
	bOpts := &buildOpts{
		base: empty.Image,
	}
	for _, o := range opts {
		o(bOpts)
	}

	// assume examples exist
	examplesExist := true
	// Get package YAML stream.
	pkgReader, err := b.pb.Init(ctx)
	if err != nil {
		return nil, nil, errors.Wrap(err, errInitBackend)
	}
	defer func() { _ = pkgReader.Close() }()

	// Get examples YAML stream.
	exReader, err := b.eb.Init(ctx)
	if err != nil && !os.IsNotExist(err) {
		return nil, nil, errors.Wrap(err, errInitBackend)
	}
	defer func() { _ = exReader.Close() }()
	// examples/ doesn't exist
	if os.IsNotExist(err) {
		examplesExist = false
	}

	// Copy stream once to parse and once write to tarball.
	pkgBuf := new(bytes.Buffer)
	pkg, err := b.pp.Parse(ctx, annotatedTeeReadCloser(pkgReader, pkgBuf))
	if err != nil {
		return nil, nil, errors.Wrap(err, errParserPackage)
	}

	metas := pkg.GetMeta()
	if len(metas) != 1 {
		return nil, nil, errors.New(errNotExactlyOneMeta)
	}

	// TODO(hasheddan): make linter selection logic configurable.
	meta := metas[0]
	var linter linter.Linter
	if meta.GetObjectKind().GroupVersionKind().Kind == pkgmetav1.ConfigurationKind {
		linter = NewConfigurationLinter()
	} else {
		linter = NewProviderLinter()
	}
	if err := linter.Lint(pkg); err != nil {
		return nil, nil, errors.Wrap(err, errLintPackage)
	}

	layers := make([]v1.Layer, 0)
	cfgFile, err := bOpts.base.ConfigFile()
	if err != nil {
		return nil, nil, errors.Wrap(err, errConfigFile)
	}

	cfg := cfgFile.Config
	cfg.Labels = make(map[string]string)

	pkgLayer, err := Layer(pkgBuf, StreamFile, PackageAnnotation, int64(pkgBuf.Len()), &cfg)
	if err != nil {
		return nil, nil, err
	}
	layers = append(layers, pkgLayer)

	// examples exist, create the layer
	if examplesExist {
		exBuf := new(bytes.Buffer)
		if _, err = b.ep.Parse(ctx, annotatedTeeReadCloser(exReader, exBuf)); err != nil {
			return nil, nil, errors.Wrap(err, errParserExample)
		}

		exLayer, err := Layer(exBuf, XpkgExamplesFile, ExamplesAnnotation, int64(exBuf.Len()), &cfg)
		if err != nil {
			return nil, nil, err
		}
		layers = append(layers, exLayer)
	}

	for _, l := range layers {
		bOpts.base, err = mutate.AppendLayers(bOpts.base, l)
		if err != nil {
			return nil, nil, errors.Wrap(err, errBuildImage)
		}
	}

	bOpts.base, err = mutate.Config(bOpts.base, cfg)
	if err != nil {
		return nil, nil, errors.Wrap(err, errMutateConfig)
	}

	return bOpts.base, meta, nil
}

// SkipContains supplies a FilterFn that skips paths that contain the give pattern.
func SkipContains(pattern string) parser.FilterFn {
	return func(path string, info os.FileInfo) (bool, error) {
		return strings.Contains(path, pattern), nil
	}
}
