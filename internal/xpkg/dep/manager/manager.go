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

package manager

import (
	"context"
	"os"

	"github.com/crossplane/crossplane/apis/pkg/v1beta1"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/spf13/afero"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/upbound/up/internal/xpkg/dep/cache"
	"github.com/upbound/up/internal/xpkg/dep/marshaler/xpkg"
	"github.com/upbound/up/internal/xpkg/dep/resolver/image"
	"github.com/upbound/up/internal/xpls/validator"
)

// Manager defines a dependency Manager
type Manager struct {
	c Cache
	i ImageResolver
	x XpkgMarshaler

	acc []*xpkg.ParsedPackage
}

// Cache defines the API contract for working with a Cache.
type Cache interface {
	Get(v1beta1.Dependency) (*xpkg.ParsedPackage, error)
	Store(v1beta1.Dependency, *xpkg.ParsedPackage) error
	Versions(v1beta1.Dependency) ([]string, error)
}

// ImageResolver defines the API contract for working with an
// ImageResolver.
type ImageResolver interface {
	ResolveDigest(context.Context, v1beta1.Dependency) (string, error)
	ResolveImage(context.Context, v1beta1.Dependency) (string, v1.Image, error)
	ResolveTag(context.Context, v1beta1.Dependency) (string, error)
}

// XpkgMarshaler defines the API contract for working with an
// xpkg.ParsedPackage marshaler.
type XpkgMarshaler interface {
	FromImage(string, string, string, v1.Image) (*xpkg.ParsedPackage, error)
	FromDir(afero.Fs, string, string, string) (*xpkg.ParsedPackage, error)
}

// New returns a new Manager
func New(opts ...Option) (*Manager, error) {
	m := &Manager{}

	c, err := cache.NewLocal()
	if err != nil {
		return nil, err
	}

	x, err := xpkg.NewMarshaler()
	if err != nil {
		return nil, err
	}

	m.i = image.NewResolver()
	m.c = c
	m.x = x
	m.acc = make([]*xpkg.ParsedPackage, 0)

	for _, o := range opts {
		o(m)
	}

	return m, nil
}

// Option modifies the Manager.
type Option func(*Manager)

// WithCache sets the supplied cache.Local on the Manager.
func WithCache(c Cache) Option {
	return func(m *Manager) {
		m.c = c
	}
}

// WithResolver sets the supplied dep.Resolver on the Manager.
func WithResolver(r ImageResolver) Option {
	return func(m *Manager) {
		m.i = r
	}
}

// Snapshot returns a Snapshot containing a view of all of the validators for
// dependencies (both defined and transitive) related to the given slice of
// v1beta1.Dependency.
func (m *Manager) Snapshot(ctx context.Context, deps []v1beta1.Dependency) (*Snapshot, error) {
	validators := make(map[schema.GroupVersionKind]validator.Validator)
	packages := make(map[string]*xpkg.ParsedPackage)

	for _, d := range deps {
		_, acc, err := m.Resolve(ctx, d)
		if err != nil {
			return nil, err
		}
		for _, p := range acc {
			for k, v := range p.Validators() {
				validators[k] = v
			}
			packages[p.Name()] = p
		}
	}

	return &Snapshot{
		view: &View{
			packages:   packages,
			validators: validators,
		},
	}, nil
}

// Versions returns the dependency versions corresponding to the supplied
// v1beta1.Dependency that currently exist locally.
func (m *Manager) Versions(ctx context.Context, d v1beta1.Dependency) ([]string, error) {
	return m.c.Versions(d)
}

// Resolve resolves the given package as well as it's transitive dependencies.
// If storage is successful, the resolved dependency is returned, errors
// otherwise.
func (m *Manager) Resolve(ctx context.Context, d v1beta1.Dependency) (v1beta1.Dependency, []*xpkg.ParsedPackage, error) {
	ud := v1beta1.Dependency{}

	e, err := m.retrievePkg(ctx, d)
	if err != nil {
		return ud, m.acc, nil
	}
	m.acc = append(m.acc, e)

	// recursively resolve all transitive dependencies
	// currently assumes we have something from
	if err := m.resolveAllDeps(ctx, e); err != nil {
		return ud, m.acc, err
	}

	ud.Type = e.Type()
	ud.Package = d.Package
	ud.Constraints = e.Version()

	return ud, m.acc, nil
}

// resolveAllDeps recursively resolves the transitive dependencies for a
// given xpkg.ParsedPackage.
func (m *Manager) resolveAllDeps(ctx context.Context, p *xpkg.ParsedPackage) error {

	if len(p.Dependencies()) == 0 {
		// no remaining dependencies to resolve
		return nil
	}

	for _, d := range p.Dependencies() {
		e, err := m.retrievePkg(ctx, d)
		if err != nil {
			return err
		}
		m.acc = append(m.acc, e)

		if err := m.resolveAllDeps(ctx, e); err != nil {
			return err
		}
	}

	return nil
}

func (m *Manager) addPkg(ctx context.Context, d v1beta1.Dependency) (*xpkg.ParsedPackage, error) {
	// this is expensive
	t, i, err := m.i.ResolveImage(ctx, d)
	if err != nil {
		return nil, err
	}

	tag, err := name.NewTag(d.Package)
	if err != nil {
		return nil, err
	}

	p, err := m.x.FromImage(tag.RegistryStr(), tag.RepositoryStr(), t, i)
	if err != nil {
		return nil, err
	}

	// add xpkg to cache
	err = m.c.Store(d, p)
	if err != nil {
		return nil, err
	}

	return p, nil
}

func (m *Manager) retrievePkg(ctx context.Context, d v1beta1.Dependency) (*xpkg.ParsedPackage, error) {
	// resolve version prior to Get
	if err := m.finalizeDepVersion(ctx, &d); err != nil {
		return nil, err
	}

	p, err := m.c.Get(d)
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}

	if os.IsNotExist(err) {
		// root dependency does not yet exist in cache, store it
		p, err = m.addPkg(ctx, d)
		if err != nil {
			return nil, err
		}
	} else {
		// check if digest is different from what we have locally
		digest, err := m.i.ResolveDigest(ctx, d)
		if err != nil {
			return nil, err
		}

		if p.Digest() != digest {
			// digest is different, update what we have
			p, err = m.addPkg(ctx, d)
			if err != nil {
				return nil, err
			}
		}
	}

	return p, nil
}

// finalizeDepVersion sets the resolved tag version on the supplied v1beta1.Dependency.
func (m *Manager) finalizeDepVersion(ctx context.Context, d *v1beta1.Dependency) error {
	// determine the version (using resolver) to use based on the supplied constraints
	v, err := m.i.ResolveTag(ctx, *d)
	if err != nil {
		return err
	}

	d.Constraints = v
	return nil
}

// Snapshot represents the dependency cache at a snapshot in time.
type Snapshot struct {
	view *View
}

// View represents the current view of the dependency cache in an easy to consume
// manner.
type View struct {
	packages   map[string]*xpkg.ParsedPackage
	validators map[schema.GroupVersionKind]validator.Validator
}

// View returns the Snapshot's View.
func (s *Snapshot) View() *View {
	return s.view
}

// Packages returns the packages map for the view.
func (v *View) Packages() map[string]*xpkg.ParsedPackage {
	return v.packages
}

// Validators returns the validators map for the view.
func (v *View) Validators() map[schema.GroupVersionKind]validator.Validator {
	return v.validators
}