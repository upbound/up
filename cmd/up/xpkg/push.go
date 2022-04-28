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
	"os"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
	"github.com/pkg/errors"
	"github.com/spf13/afero"

	"github.com/upbound/up/internal/credhelper"
	"github.com/upbound/up/internal/xpkg"
)

const (
	errGetwd           = "failed to get working directory while searching for package"
	errFindPackageinWd = "failed to find a package in current working directory"
)

const upboundRegistry = "registry.upbound.io"

// AfterApply constructs and binds Upbound-specific context to any subcommands
// that have Run() methods that receive it.
func (c *pushCmd) AfterApply() error {
	c.fs = afero.NewOsFs()
	return nil
}

// pushCmd pushes a package.
type pushCmd struct {
	fs afero.Fs

	Tag     string `arg:"" help:"Tag of the package to be pushed. Must be a valid OCI image tag."`
	Package string `short:"f" help:"Path to package. If not specified and only one package exists in current directory it will be used."`
	Profile string `env:"UP_PROFILE" help:"Profile used to execute command."`
}

// Run runs the push cmd.
func (c *pushCmd) Run() error {
	tag, err := name.NewTag(c.Tag, name.WithDefaultRegistry(upboundRegistry))
	if err != nil {
		return err
	}

	// If package is not defined, attempt to find single package in current
	// directory.
	if c.Package == "" {
		wd, err := os.Getwd()
		if err != nil {
			return errors.Wrap(err, errGetwd)
		}
		path, err := xpkg.FindXpkgInDir(c.fs, wd)
		if err != nil {
			return errors.Wrap(err, errFindPackageinWd)
		}
		c.Package = path
	}
	img, err := tarball.ImageFromPath(c.Package, nil)
	if err != nil {
		return err
	}
	if err := remote.Write(tag, img, remote.WithAuthFromKeychain(
		authn.NewMultiKeychain(
			authn.NewKeychainFromHelper(
				credhelper.New(credhelper.WithProfile(c.Profile)),
			),
			authn.DefaultKeychain,
		),
	)); err != nil {
		return err
	}
	return nil
}
