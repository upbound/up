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
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/afero"

	v1ext "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	v1beta1ext "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/yaml"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	xpv1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
	"github.com/crossplane/crossplane/apis/apiextensions/v1beta1"

	"github.com/upbound/up/internal/xpkg"
	rxpkg "github.com/upbound/up/internal/xpkg/dep/resolver/xpkg"
)

const (
	crdNameFmt = "%s.yaml"

	errFailedToCreateMeta     = "failed to create meta file in entry"
	errFailedToCreateCRD      = "failed to create crd"
	errNoObjectsToFlushToDisk = "no objects to flush"
	errObjectNotCRDNorXRD     = "object is not a crd"
)

// entry is the internal representation of the cache at a given directory
type entry struct {
	cacheRoot string
	fs        afero.Fs
	path      string
	pkg       *rxpkg.ParsedPackage
}

// NewEntry --
// TODO(@tnthornton) maybe pull this into cache.go
func (c *Local) newEntry(p *rxpkg.ParsedPackage) *entry {

	return &entry{
		cacheRoot: c.root,
		fs:        c.fs,
		pkg:       p,
	}
}

// CurrentEntry retrieves the current Entry at the given path.
// TODO(@tnthornton) maybe pull this into cache.go
func (c *Local) currentEntry(path string) (*entry, error) {

	e := &entry{
		cacheRoot: c.root,
		fs:        c.fs,
		path:      path,
	}

	// grab the current entry if it exists
	pkg, err := c.pkgres.FromDir(c.fs, e.location())
	if os.IsNotExist(err) {
		return e, err
	}
	if err != nil {
		return nil, err
	}

	e.pkg = pkg

	return e, nil
}

// // SetDigest sets the digest for the entry.
func (e *entry) setDigest() error {
	if e.pkg == nil {
		return errors.New(errNoObjectsToFlushToDisk)
	}
	// writing empty digest file
	_, err := e.fs.Create(filepath.Join(e.location(), e.pkg.Digest()))
	if err != nil {
		return err
	}

	return nil
}

// flush writes the package contents to disk.
// In addition to error, flush returns the number of meta, CRD, and XRD files
// written to the entry on disk.
func (e *entry) flush() (*flushstats, error) {
	stats := &flushstats{}

	if e.pkg == nil {
		return stats, errors.New(errNoObjectsToFlushToDisk)
	}

	metaStats, err := e.writeMeta(e.pkg.Meta())
	if err != nil {
		return stats, err
	}

	stats.combine(metaStats)

	objstats, err := e.writeObjects(e.pkg.Objects())
	if err != nil {
		return stats, err
	}

	stats.combine(objstats)

	return stats, err
}

// writeMeta writes the meta file to disk.
// If the meta file was written, we return the file count
func (e *entry) writeMeta(o runtime.Object) (*flushstats, error) {
	stats := &flushstats{}

	cf, err := e.fs.Create(filepath.Join(e.location(), xpkg.MetaFile))
	if err != nil {
		return stats, errors.Wrap(err, errFailedToCreateMeta)
	}
	defer cf.Close() // nolint:errcheck

	b, err := yaml.Marshal(o)
	if err != nil {
		return stats, errors.Wrap(err, errFailedToCreateMeta)
	}

	mb, err := cf.Write(b)

	if mb > 0 {
		stats.incMetas()
		return stats, err
	}
	return stats, err
}

// writeObjects writes out the CRDs and XRDs that came from the package.yaml
func (e *entry) writeObjects(objs []runtime.Object) (*flushstats, error) { // nolint:gocyclo
	stats := &flushstats{}

	for _, o := range objs {
		b, err := yaml.Marshal(o)
		if err != nil {
			return stats, err
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

		// TODO(@tnthornton) add support for compositions

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
			return stats, errors.New(errObjectNotCRDNorXRD)
		}

		crdf, err := e.fs.Create(filepath.Join(e.location(), fmt.Sprintf(crdNameFmt, name)))
		if err != nil {
			return stats, err
		}
		defer crdf.Close() // nolint:errcheck

		crdb, err := crdf.Write(b)
		if err != nil {
			return stats, err
		}

		if crdb == 0 {
			return stats, errors.New(errFailedToCreateCRD)
		}
		if isXRD {
			stats.incXRDs()
		} else {
			stats.incCRDs()
		}
	}

	return stats, nil
}

// Path returns the path this entry represents.
func (e *entry) Path() string {
	return e.path
}

// SetPath sets the Entry path to the supplied path.
func (e *entry) setPath(path string) {
	e.path = path
}

// Clean cleans all files from the entry without deleting the parent directory
// where the Entry is located.
func (e *entry) Clean() error {
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

func (e *entry) location() string {
	return filepath.Join(e.cacheRoot, e.path)
}

type flushstats struct {
	// comps int
	crds  int
	metas int
	xrds  int
}

// func (s *flushstats) incComps() {
// 	s.comps++
// }

func (s *flushstats) incCRDs() {
	s.crds++
}

func (s *flushstats) incMetas() {
	s.metas++
}

func (s *flushstats) incXRDs() {
	s.xrds++
}

func (s *flushstats) combine(src *flushstats) {
	// s.comps += src.comps
	s.crds += src.crds
	s.metas += src.metas
	s.xrds += src.xrds
}
