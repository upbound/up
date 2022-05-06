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
	errBuildImage    = "failed to build image from layers"
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

func (t *teeReader) Annotate() interface{} {
	anno, ok := t.r.(parser.AnnotatedReadCloser)
	if !ok {
		return nil
	}
	return anno.Annotate()
}

// Build compiles a Crossplane package from an on-disk package.
// TODO(tnthornton) we should work towards cleaning up this API. It feels
// pretty clunky at the moment.
func Build(ctx context.Context, pkgBackend, exBackend parser.Backend, p parser.Parser, e *examples.Parser) (v1.Image, runtime.Object, error) { // nolint:gocyclo
	// assume examples exist
	examplesExist := true
	// Get package YAML stream.
	pkgReader, err := pkgBackend.Init(ctx)
	if err != nil {
		return nil, nil, errors.Wrap(err, errInitBackend)
	}
	defer func() { _ = pkgReader.Close() }()

	// Get examples YAML stream.
	exReader, err := exBackend.Init(ctx)
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
	pkg, err := p.Parse(ctx, annotatedTeeReadCloser(pkgReader, pkgBuf))
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

	addendums := make([]mutate.Addendum, 0)

	pkgAddendum, err := PackageAddendum(pkgBuf)
	if err != nil {
		return nil, nil, err
	}
	addendums = append(addendums, pkgAddendum)

	// examples exist, create the layer
	if examplesExist {
		exBuf := new(bytes.Buffer)
		_, err = e.Parse(ctx, annotatedTeeReadCloser(exReader, exBuf))
		if err != nil {
			return nil, nil, errors.Wrap(err, errParserExample)
		}

		exAddendum, err := ExamplesAddendum(exBuf)
		if err != nil {
			return nil, nil, err
		}
		addendums = append(addendums, exAddendum)
	}

	img := empty.Image
	for _, a := range addendums {
		img, err = mutate.Append(img, a)
		if err != nil {
			return nil, nil, errors.Wrap(err, errBuildImage)
		}
	}
	return img, meta, nil
}

// SkipContains supplies a FilterFn that skips paths that contain the give pattern.
func SkipContains(pattern string) parser.FilterFn {
	return func(path string, info os.FileInfo) (bool, error) {
		return strings.Contains(path, pattern), nil
	}
}
