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

package dep

import (
	"bufio"
	"context"
	encjson "encoding/json"
	"io"
	"os"
	"path/filepath"

	"github.com/pkg/errors"

	"github.com/spf13/afero"
	sigsyaml "sigs.k8s.io/yaml"

	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer/json"
	"k8s.io/apimachinery/pkg/util/yaml"

	"github.com/crossplane/crossplane-runtime/pkg/parser"
	v1 "github.com/crossplane/crossplane/apis/pkg/meta/v1"

	"github.com/upbound/up/internal/xpkg"
)

// TODO(@tnthornton) there are a few errors below that are copied from other
// packages due to the errors not being exposed publicly. We should consider
// how we want to account for this. Most likely it makes the most sense
// to have the code that is expressing these errors refactored to be more
// general so we can reuse it here.
const (
	errInitBackend          = "could not initialize filesystem backend"
	errInvalidMetaFile      = "invalid meta type supplied"
	errMetaFileDoesNotExist = "meta file does not exist"
	errNotExactlyOneMeta    = "not exactly one package meta type"
	errNotMeta              = "meta type is not a package"
	errParserPackage        = "failed to parse package"
)

// Workspace defines our view of the current directory
type Workspace struct {
	fs             afero.Fs
	fsBackend      *parser.FsBackend
	metaFileExists bool
	wd             string
}

// NewWorkspace establishees a workspace for acting on current ws entries
func NewWorkspace(fs afero.Fs) *Workspace {
	wd, _ := os.Getwd()

	return &Workspace{
		fs: fs,
		wd: wd,
	}
}

// Init initializes the workspace
func (w *Workspace) Init() error {
	exists, err := afero.Exists(w.fs, filepath.Join(w.wd, xpkg.MetaFile))
	if err != nil {
		return errors.Wrap(err, errMetaFileDoesNotExist)
	}

	backend := parser.NewFsBackend(
		w.fs,
		parser.FsDir(w.wd),
		parser.FsFilters(
			parser.SkipDirs(),
			parser.SkipNotYAML(),
			parser.SkipEmpty(),
		),
	)

	w.fsBackend = backend
	w.metaFileExists = exists

	return nil
}

// MetaExists returns true if a meta file exists in the workspace
func (w *Workspace) MetaExists() bool {
	return w.metaFileExists
}

// Upsert will add an entry to the meta file, if the meta file exists and
// does not yet have an entry for the given package. If an entry does exist,
// the entry will be updated to the given package version.
func (w *Workspace) Upsert(d v1.Dependency) error {
	p, err := w.readPkgMeta()
	if err != nil {
		return err
	}

	err = upsertDeps(d, p)
	if err != nil {
		return err
	}

	return w.writeMetaPkg(p)
}

func upsertDeps(d v1.Dependency, p v1.Pkg) error {
	deps := p.GetDependencies()

	seen := false
	for i := range deps {
		// modify the underlying slice
		if *deps[i].Provider == *d.Provider {
			deps[i].Version = d.Version
			seen = true
		}
	}

	if !seen {
		deps = append(deps, v1.Dependency{
			Provider: d.Provider,
			Version:  d.Version,
		})
	}

	switch v := p.(type) {
	case *v1.Configuration:
		v.Spec.DependsOn = deps
	case *v1.Provider:
		v.Spec.DependsOn = deps
	}

	return nil
}

func (w *Workspace) readPkgMeta() (v1.Pkg, error) {
	// Get YAML stream.
	r, err := w.fsBackend.Init(context.Background())

	if err != nil {
		return nil, errors.Wrap(err, errInitBackend)
	}

	defer func() { _ = r.Close() }()

	metas, err := parse(r)
	if err != nil {
		return nil, errors.Wrap(err, errParserPackage)
	}

	if len(metas) != 1 {
		return nil, errors.New(errNotExactlyOneMeta)
	}

	o := metas[0]

	pkg, ok := xpkg.TryConvertToPkg(o, &v1.Provider{}, &v1.Configuration{})
	if !ok {
		return nil, errors.New(errNotMeta)
	}

	return pkg, nil
}

// writeMetaPkg writes to the current meta file (crossplane.yaml).
// If the file currently exists, it will be overwritten rather than
// appended to.
func (w *Workspace) writeMetaPkg(p v1.Pkg) error {
	data, err := sigsyaml.Marshal(p)
	if err != nil {
		return err
	}

	// (@tnthornton) workaround for `creationTimestamp: null` in marshaled result.
	// see https://github.com/kubernetes/kubernetes/pull/104857 for inspiration

	t := apimetav1.Time{}

	switch v := p.(type) {
	case *v1.Configuration:
		t = v.GetCreationTimestamp()
	case *v1.Provider:
		t = v.GetCreationTimestamp()
	default:
		return errors.New(errInvalidMetaFile)
	}

	if t.Equal(&apimetav1.Time{}) {
		// the timestamp is empty, we need to clean it from the resulting
		// file data
		data, err = cleanNullTs(p)
		if err != nil {
			return err
		}
	}

	return afero.WriteFile(w.fs, filepath.Join(w.wd, xpkg.MetaFile), data, os.ModePerm)
}

// cleanNullTs is a helper function for cleaning the erroneous
// `creationTimestamp: null` from the marshaled data that we're
// going to writer to the meta file.
func cleanNullTs(p v1.Pkg) ([]byte, error) {
	ob, err := encjson.Marshal(p)
	if err != nil {
		return nil, err
	}
	var m map[string]interface{}
	err = encjson.Unmarshal(ob, &m)
	if err != nil {
		return nil, err
	}
	// remove the erroneous creationTimestamp: null entry
	delete(m["metadata"].(map[string]interface{}), "creationTimestamp")

	return sigsyaml.Marshal(m)
}

// parse attempts to decode objects recognized by the meta scheme.
// ref: https://github.com/crossplane/crossplane-runtime/blob/master/pkg/parser/parser.go#L94
// current impl does too much for our use case (parses beyond the meta file).
//
// @TODO(@tnthornton) see if we can find a happy middle ground between the two
// impls to reduce duplication.
func parse(reader io.ReadCloser) ([]runtime.Object, error) { //nolint:gocyclo
	defer func() { _ = reader.Close() }()

	metaScheme, err := xpkg.BuildMetaScheme()
	if err != nil {
		return nil, err
	}
	yr := yaml.NewYAMLReader(bufio.NewReader(reader))
	dm := json.NewSerializerWithOptions(
		json.DefaultMetaFactory,
		metaScheme,
		metaScheme,
		json.SerializerOptions{Yaml: true},
	)

	metas := []runtime.Object{}

	for {
		bytes, err := yr.Read()
		if err != nil && err != io.EOF {
			return nil, err
		}
		if err == io.EOF {
			break
		}
		if len(bytes) == 0 {
			continue
		}
		m, _, _ := dm.Decode(bytes, nil, nil)
		if m != nil {
			metas = append(metas, m)
		}
	}
	return metas, nil
}
