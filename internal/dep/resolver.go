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
	"context"
	"sort"

	"github.com/Masterminds/semver"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/pkg/errors"

	"github.com/crossplane/crossplane/apis/pkg/v1beta1"
)

const (
	defaultVer = ">=v0.0.0"

	errInvalidConstraint  = "invalid dependency constraint"
	errInvalidProviderRef = "invalid provider reference"
	errFailedToFetchTags  = "failed to fetch tags"
	errNoMatchingVersion  = "supplied version does not match an existing version"
)

// Resolver --
type Resolver struct {
	f fetcher
}

// NewResolver --
func NewResolver(opts ...ResolverOption) *Resolver {
	r := &Resolver{}
	for _, o := range opts {
		o(r)
	}
	return r
}

// ResolverOption modifies the dependency resolver.
type ResolverOption func(*Resolver)

// WithFetcher --
func WithFetcher(f fetcher) ResolverOption {
	return func(r *Resolver) {
		r.f = f
	}
}

// ResolveImage --
func (r *Resolver) ResolveImage(ctx context.Context, dep v1beta1.Dependency) (v1.Image, error) {

	tag, err := r.ResolveTag(ctx, dep)
	if err != nil {
		return nil, err
	}

	remoteImageRef, err := name.ParseReference(ImgTag(v1beta1.Dependency{
		Package:     dep.Package,
		Type:        dep.Type,
		Constraints: tag,
	}))
	if err != nil {
		return nil, err
	}

	return r.f.Fetch(ctx, remoteImageRef)
}

// ResolveTag --
func (r *Resolver) ResolveTag(ctx context.Context, dep v1beta1.Dependency) (string, error) {
	// if the passed in version was blank use the default to pass
	// constraint checks and grab latest semver
	if dep.Constraints == "" {
		dep.Constraints = defaultVer
	}

	c, err := semver.NewConstraint(dep.Constraints)
	if err != nil {
		return "", errors.Wrap(err, errInvalidConstraint)
	}

	ref, err := name.ParseReference(dep.Identifier())
	if err != nil {
		return "", errors.Wrap(err, errInvalidProviderRef)
	}

	tags, err := r.f.Tags(ctx, ref)
	if err != nil {
		return "", errors.Wrap(err, errFailedToFetchTags)
	}

	vs := []*semver.Version{}
	for _, r := range tags {
		v, err := semver.NewVersion(r)
		if err != nil {
			// We skip any tags that are not valid semantic versions.
			//
			// TODO @(tnthornton) we should verify this is the behavior we
			// want long term - i.e. should we care if an end user chooses
			// not to tag their packages with semver?
			continue
		}
		vs = append(vs, v)
	}

	sort.Sort(semver.Collection(vs))
	var ver string
	for _, v := range vs {
		if c.Check(v) {
			ver = v.Original()
		}
	}

	if ver == "" {
		return "", errors.New(errNoMatchingVersion)
	}

	return ver, nil
}
