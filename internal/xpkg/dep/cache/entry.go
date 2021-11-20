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
	"archive/tar"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/spf13/afero"
	"github.com/spf13/afero/tarfs"

	v1ext "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	v1beta1ext "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/yaml"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/parser"
	xpv1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
	"github.com/crossplane/crossplane/apis/apiextensions/v1beta1"
	metav1 "github.com/crossplane/crossplane/apis/pkg/meta/v1"
	pkgv1beta1 "github.com/crossplane/crossplane/apis/pkg/v1beta1"

	"github.com/upbound/up/internal/xpkg"
)

const (
	digestPrefix = "sha256"
	crdNameFmt   = "%s.yaml"

	errBuildMetaScheme   = "failed to build meta scheme for package parser"
	errBuildObjectScheme = "failed to build object scheme for package parser"

	errFailedEntryCreate    = "failed to create entry"
	errFailedToCreateMeta   = "failed to create meta file in entry"
	errFailedToCreateCRD    = "failed to create crd"
	errFailedToParsePkgYaml = "failed to parse package yaml"
	errLintPackage          = "failed to lint package"

	errNotExactlyOneMeta  = "not exactly one package meta type"
	errObjectNotCRDNorXRD = "object is not a crd"
)

// Entry is the internal representation of the cache at a given directory
type Entry struct {
	cacheRoot string
	fs        afero.Fs
	meta      runtime.Object
	path      string
	pkg       *parser.Package
	pkgType   pkgv1beta1.PackageType
	sha       string
}

// NewEntry --
// TODO maybe pull this into cache.go
func (c *Local) NewEntry(i v1.Image) (*Entry, error) {
	d, err := i.Digest()
	if err != nil {
		return nil, errors.Wrap(err, errFailedEntryCreate)
	}

	r := mutate.Extract(i)
	fs := tarfs.New(tar.NewReader(r))
	pkgYaml, err := fs.Open(xpkg.StreamFile)
	if err != nil {
		return nil, errors.Wrap(err, errOpenPackageStream)
	}

	pkg, pt, err := c.parse(pkgYaml)
	if err != nil {
		return nil, err
	}

	return &Entry{
		cacheRoot: c.root,
		fs:        c.fs,
		// pkg.meta has a len of 1 otherwise we would have an error from parse
		meta:    pkg.GetMeta()[0],
		pkg:     pkg,
		pkgType: pt,
		sha:     d.String(),
	}, nil
}

// TODO maybe pull this into cache.go

// CurrentEntry retrieves the current Entry at the given path.
func (c *Local) CurrentEntry(path string) (*Entry, error) {

	e := &Entry{
		cacheRoot: c.root,
		fs:        c.fs,
		path:      path,
	}

	// grab the files from the directory
	files, err := afero.ReadDir(c.fs, e.location())
	if os.IsNotExist(err) {
		return e, err
	}
	if err != nil {
		return nil, err
	}
	for _, f := range files {
		if strings.HasPrefix(f.Name(), digestPrefix) {
			e.sha = f.Name()
			continue
		}
		if f.Name() == xpkg.MetaFile {
			m, err := c.fs.Open(filepath.Join(e.location(), f.Name()))
			if err != nil {
				return nil, err
			}

			// pkg.meta has a len of 1 otherwise we would have an error from parse
			pkg, pt, err := c.parse(m)
			if err != nil {
				return nil, err
			}

			e.meta = pkg.GetMeta()[0]
			e.pkg = pkg
			e.pkgType = pt
			continue
		}
	}
	return e, nil
}

func (c *Local) parse(r io.ReadCloser) (*parser.Package, pkgv1beta1.PackageType, error) {
	var pkgType pkgv1beta1.PackageType
	// parse package.yaml
	pkg, err := c.parser.Parse(context.Background(), r)
	if err != nil {
		return nil, pkgType, errors.Wrap(err, errFailedToParsePkgYaml)
	}

	metas := pkg.GetMeta()
	if len(metas) != 1 {
		return nil, pkgType, errors.New(errNotExactlyOneMeta)
	}

	meta := metas[0]
	var linter parser.Linter
	if meta.GetObjectKind().GroupVersionKind().Kind == metav1.ConfigurationKind {
		linter = xpkg.NewConfigurationLinter()
		pkgType = pkgv1beta1.ConfigurationPackageType
	} else {
		linter = xpkg.NewProviderLinter()
		pkgType = pkgv1beta1.ProviderPackageType
	}
	if err := linter.Lint(pkg); err != nil {
		return nil, pkgType, errors.Wrap(err, errLintPackage)
	}

	return pkg, pkgType, nil
}

// Digest returns the current SHA digest filename for the entry.
func (e *Entry) Digest() string {
	// maybe resolve the digest from the file as part of this?
	return e.sha
}

// SetDigest sets the digest for the entry.
func (e *Entry) setDigest() error {
	// writing empty digest file
	_, err := e.fs.Create(filepath.Join(e.location(), e.sha))
	if err != nil {
		return err
	}

	return nil
}

// Meta returns the parsed meta object for an entry.
func (e *Entry) Meta() runtime.Object {
	return e.meta
}

// Objects returns the slice of parsed objects for an entry. The slice can
// contain CRDs or XRDs depending on the package type.
func (e *Entry) Objects() []runtime.Object {
	if e.pkg != nil {
		return e.pkg.GetObjects()
	}
	return []runtime.Object{}
}

// Type returns the pkgv1beta1.PackageType of package for this entry.
func (e *Entry) Type() pkgv1beta1.PackageType {
	return e.pkgType
}

// flush writes the package contents to disk.
// In addition to error, flush returns the number of meta, CRD, and XRD files
// written to the entry on disk.
func (e *Entry) flush() (int, int, int, error) {

	mNum, err := e.writeMeta(e.Meta())
	if err != nil {
		return mNum, 0, 0, err
	}
	cNum, xNum, err := e.writeObjects(e.Objects())
	if err != nil {
		return mNum, cNum, xNum, err
	}

	return mNum, cNum, xNum, err
}

// writeMeta writes the meta file to disk.
// If the meta file was written, we return the file count
func (e *Entry) writeMeta(o runtime.Object) (int, error) {
	cf, err := e.fs.Create(filepath.Join(e.location(), xpkg.MetaFile))
	if err != nil {
		return 0, errors.Wrap(err, errFailedToCreateMeta)
	}
	defer cf.Close() // nolint:errcheck

	b, err := yaml.Marshal(o)
	if err != nil {
		return 0, errors.Wrap(err, errFailedToCreateMeta)
	}

	mb, err := cf.Write(b)

	if mb > 0 {
		return 1, err
	}
	return 0, err
}

// writeObjects writes out the CRDs and XRDs that came from the package.yaml
func (e *Entry) writeObjects(objs []runtime.Object) (int, int, error) { // nolint:gocyclo

	crdc := 0
	xrdc := 0
	for _, o := range objs {
		b, err := yaml.Marshal(o)
		if err != nil {
			return crdc, xrdc, err
		}

		isXRD := false

		if err := xpkg.IsCRD(o); err != nil {
			if err := xpkg.IsXRD(o); err != nil {
				// not a CRD nor an XRD, skip
				continue
			} else {
				isXRD = true
			}
		}

		name := ""
		switch crd := o.(type) {
		case *v1beta1ext.CustomResourceDefinition:
			name = crd.GetName()
		case *v1ext.CustomResourceDefinition:
			name = crd.GetName()
		case *v1beta1.CompositeResourceDefinition:
			name = crd.GetName()
		case *xpv1.CompositeResourceDefinition:
			name = crd.GetName()
		default:
			return 0, 0, errors.New(errObjectNotCRDNorXRD)
		}

		crdf, err := e.fs.Create(filepath.Join(e.location(), fmt.Sprintf(crdNameFmt, name)))
		if err != nil {
			return crdc, xrdc, err
		}
		defer crdf.Close() // nolint:errcheck

		crdb, err := crdf.Write(b)
		if err != nil {
			return crdc, xrdc, err
		}

		if crdb == 0 {
			return crdc, xrdc, errors.New(errFailedToCreateCRD)
		}
		if isXRD {
			xrdc++
		} else {
			crdc++
		}
	}

	return crdc, xrdc, nil
}

// Path returns the path this entry represents.
func (e *Entry) Path() string {
	return e.path
}

// SetPath sets the Entry path to the supplied path.
func (e *Entry) setPath(path string) {
	e.path = path
}

// Clean cleans all files from the entry without deleting the parent directory
// where the Entry is located.
func (e *Entry) Clean() error {
	files, err := afero.ReadDir(e.fs, e.location())
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	for _, f := range files {
		if err := e.fs.RemoveAll(filepath.Join(e.location(), f.Name())); err != nil {
			return err
		}
	}
	return nil
}

func (e *Entry) location() string {
	return filepath.Join(e.cacheRoot, e.path)
}
