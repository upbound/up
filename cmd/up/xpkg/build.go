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
	"context"
	"path/filepath"

	"github.com/crossplane/crossplane-runtime/pkg/parser"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
	"github.com/pkg/errors"
	"github.com/spf13/afero"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/upbound/up/internal/xpkg"
	"github.com/upbound/up/internal/xpkg/parser/examples"
	"github.com/upbound/up/internal/xpkg/parser/yaml"
)

const (
	errGetNameFromMeta = "failed to get name from crossplane.yaml"
	errBuildPackage    = "failed to build package"
	errImageDigest     = "failed to get package digest"
	errCreatePackage   = "failed to create package file"

	examplesDir = "examples/"
)

// AfterApply constructs and binds Upbound-specific context to any subcommands
// that have Run() methods that receive it.
func (c *buildCmd) AfterApply() error {
	c.fs = afero.NewOsFs()

	root, err := filepath.Abs(c.PackageRoot)
	if err != nil {
		return err
	}
	c.root = root

	ex, err := filepath.Abs(c.ExamplesRoot)
	if err != nil {
		return err
	}

	pp, err := yaml.New()
	if err != nil {
		return err
	}

	c.builder = xpkg.New(
		parser.NewFsBackend(
			c.fs,
			parser.FsDir(root),
			parser.FsFilters(
				append(
					buildFilters(root, c.Ignore),
					xpkg.SkipContains(examplesDir))...),
		),
		parser.NewFsBackend(
			c.fs,
			parser.FsDir(ex),
			parser.FsFilters(
				buildFilters(ex, c.Ignore)...),
		),
		pp,
		examples.New(),
	)

	return nil
}

// buildCmd builds a crossplane package.
type buildCmd struct {
	fs      afero.Fs
	builder *xpkg.Builder
	root    string

	Name string `optional:"" help:"Name of the package to be built. Uses name in crossplane.yaml if not specified. Does not correspond to package tag."`

	PackageRoot  string   `short:"f" help:"Path to package directory." default:"."`
	ExamplesRoot string   `short:"e" help:"Path to package examples directory." default:"./examples"`
	Ignore       []string `help:"Paths, specified relative to --package-root, to exclude from the package."`
}

// Run executes the build command.
func (c *buildCmd) Run() error { //nolint:gocyclo

	img, meta, err := c.builder.Build(context.Background())
	if err != nil {
		return errors.Wrap(err, errBuildPackage)
	}

	hash, err := img.Digest()
	if err != nil {
		return errors.Wrap(err, errImageDigest)
	}

	pkgName := c.Name
	if pkgName == "" {
		pkgMeta, ok := meta.(metav1.Object)
		if !ok {
			return errors.New(errGetNameFromMeta)
		}
		pkgName = xpkg.FriendlyID(pkgMeta.GetName(), hash.Hex)
	}

	f, err := c.fs.Create(xpkg.BuildPath(c.root, pkgName))
	if err != nil {
		return errors.Wrap(err, errCreatePackage)
	}

	defer func() { _ = f.Close() }()
	if err := tarball.Write(nil, img, f); err != nil {
		return err
	}
	return nil
}

// default build filters skip directories, empty files, and files without YAML
// extension in addition to any paths specified.
func buildFilters(root string, skips []string) []parser.FilterFn {
	defaultFns := []parser.FilterFn{
		parser.SkipDirs(),
		parser.SkipNotYAML(),
		parser.SkipEmpty(),
	}
	opts := make([]parser.FilterFn, len(skips)+len(defaultFns))
	copy(opts, defaultFns)
	for i, s := range skips {
		opts[i+len(defaultFns)] = parser.SkipPath(filepath.Join(root, s))
	}
	return opts
}
