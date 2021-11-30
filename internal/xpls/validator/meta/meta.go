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

package meta

import (
	"bytes"
	"context"
	"errors"
	"io/ioutil"

	"sigs.k8s.io/yaml"

	"github.com/crossplane/crossplane-runtime/pkg/parser"
	pkgmetav1 "github.com/crossplane/crossplane/apis/pkg/meta/v1"

	"github.com/upbound/up/internal/xpkg"
	"github.com/upbound/up/internal/xpkg/dep/manager"
	mxpkg "github.com/upbound/up/internal/xpkg/dep/marshaler/xpkg"
	xpkgparser "github.com/upbound/up/internal/xpkg/parser"
)

var (
	dependsOnPathFmt = "spec.dependsOn[%d].%s"

	errFailedConvertToPkg = "unable to convert to package"
)

// Meta defines a validator for meta files.
type Meta struct {
	manager *manager.Manager
	p       *parser.PackageParser
	// TODO(@tnthornton) move to accepting a snapshot rather than the map
	// once Snapshots are first class citizens.
	packages map[string]*mxpkg.ParsedPackage
}

// New returns a new Meta validator.
func New(m *manager.Manager, pkgs map[string]*mxpkg.ParsedPackage) (*Meta, error) {
	p, err := xpkgparser.New()
	if err != nil {
		return nil, err
	}

	return &Meta{
		manager:  m,
		p:        p,
		packages: pkgs,
	}, nil
}

// Marshal marshals the given data object into a Pkg, errors otherwise.
func (m *Meta) Marshal(data interface{}) (pkgmetav1.Pkg, error) {
	b, err := yaml.Marshal(data)
	if err != nil {
		return nil, err
	}

	// convert data to a package
	ppkg, err := m.p.Parse(context.Background(), ioutil.NopCloser(bytes.NewReader(b)))
	if err != nil {
		return nil, err
	}

	if len(ppkg.GetMeta()) != 1 {
		return nil, err
	}

	pkg, ok := xpkg.TryConvertToPkg(ppkg.GetMeta()[0], &pkgmetav1.Provider{}, &pkgmetav1.Configuration{})
	if !ok {
		return nil, errors.New(errFailedConvertToPkg)
	}
	return pkg, nil
}
