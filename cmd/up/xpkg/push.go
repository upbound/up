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
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/alecthomas/kong"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
	"github.com/pkg/errors"
	"github.com/pterm/pterm"
	"github.com/spf13/afero"
	"golang.org/x/sync/errgroup"

	"github.com/upbound/up-sdk-go/service/repositories"
	"github.com/upbound/up/internal/credhelper"
	"github.com/upbound/up/internal/upbound"
	"github.com/upbound/up/internal/xpkg"
)

const (
	errCreateNotUpbound  = "cannot create repository for non-Upbound registry"
	errCreateAccountRepo = "cannot create repository without account and repository names"
	errCreateRepo        = "failed to create repository"
	errGetwd             = "failed to get working directory while searching for package"
	errFindPackageinWd   = "failed to find a package in current working directory"
	errBuildImage        = "failed to build image from layers"
)

// AfterApply constructs and binds Upbound-specific context to any subcommands
// that have Run() methods that receive it.
func (c *pushCmd) AfterApply(kongCtx *kong.Context) error {
	c.fs = afero.NewOsFs()
	upCtx, err := upbound.NewFromFlags(c.Flags)
	if err != nil {
		return err
	}
	kongCtx.Bind(upCtx)
	return nil
}

// pushCmd pushes a package.
type pushCmd struct {
	fs afero.Fs

	Tag     string   `arg:"" help:"Tag of the package to be pushed. Must be a valid OCI image tag."`
	Package []string `short:"f" help:"Path to packages. If not specified and only one package exists in current directory it will be used."`
	Create  bool     `help:"Create repository on push if it does not exist."`

	// Common Upbound API configuration
	Flags upbound.Flags `embed:""`
}

// Run runs the push cmd.
func (c *pushCmd) Run(p pterm.TextPrinter, upCtx *upbound.Context) error { //nolint:gocyclo
	tag, err := name.NewTag(c.Tag, name.WithDefaultRegistry(upCtx.RegistryEndpoint.Hostname()))
	if err != nil {
		return err
	}

	if c.Create {
		if !strings.Contains(tag.RegistryStr(), upCtx.RegistryEndpoint.Hostname()) {
			return errors.New(errCreateNotUpbound)
		}
		parts := strings.Split(tag.RepositoryStr(), "/")
		if len(parts) != 2 {
			return errors.New(errCreateAccountRepo)
		}
		cfg, err := upCtx.BuildSDKConfig(upCtx.Profile.Session)
		if err != nil {
			return err
		}
		if err := repositories.NewClient(cfg).CreateOrUpdate(context.Background(), parts[0], parts[1]); err != nil {
			return errors.Wrap(err, errCreateRepo)
		}
	}

	// If package is not defined, attempt to find single package in current
	// directory.
	if len(c.Package) == 0 {
		wd, err := os.Getwd()
		if err != nil {
			return errors.Wrap(err, errGetwd)
		}
		path, err := xpkg.FindXpkgInDir(c.fs, wd)
		if err != nil {
			return errors.Wrap(err, errFindPackageinWd)
		}
		c.Package = []string{path}
	}

	kc := authn.NewMultiKeychain(
		authn.NewKeychainFromHelper(
			credhelper.New(
				credhelper.WithDomain(upCtx.Domain.Hostname()),
				credhelper.WithProfile(c.Flags.Profile),
			),
		),
		authn.DefaultKeychain,
	)

	adds := make([]mutate.IndexAddendum, len(c.Package))

	// NOTE(hasheddan): the errgroup context is passed to each image write,
	// meaning that if one fails it will cancel others that are in progress.
	g, ctx := errgroup.WithContext(context.Background())
	for i, x := range c.Package {
		// pin range variables for use in go func
		i, x := i, x
		g.Go(func() error {
			img, err := tarball.ImageFromPath(filepath.Clean(x), nil)
			if err != nil {
				return err
			}

			// annotate image layers
			aimg, err := annotate(img)
			if err != nil {
				return err
			}

			var t name.Reference = tag
			if len(c.Package) > 1 {
				d, err := aimg.Digest()
				if err != nil {
					return err
				}
				t, err = name.NewDigest(fmt.Sprintf("%s@%s", tag.Repository.Name(), d.String()), name.WithDefaultRegistry(upCtx.RegistryEndpoint.Hostname()))
				if err != nil {
					return err
				}

				mt, err := aimg.MediaType()
				if err != nil {
					return err
				}

				conf, err := aimg.ConfigFile()
				if err != nil {
					return err
				}

				adds[i] = mutate.IndexAddendum{
					Add: aimg,
					Descriptor: v1.Descriptor{
						MediaType: mt,
						Platform: &v1.Platform{
							Architecture: conf.Architecture,
							OS:           conf.OS,
							OSVersion:    conf.OSVersion,
						},
					},
				}
			}
			if err := remote.Write(t, aimg, remote.WithAuthFromKeychain(kc), remote.WithContext(ctx)); err != nil {
				return err
			}
			return nil
		})
	}

	// Error if writing any images failed.
	if err := g.Wait(); err != nil {
		return err
	}

	// If we pushed more than one xpkg then we need to write index.
	if len(c.Package) > 1 {
		if err := remote.WriteIndex(tag, mutate.AppendManifests(empty.Index, adds...), remote.WithAuthFromKeychain(kc)); err != nil {
			return err
		}
	}

	p.Printfln("xpkg pushed to %s", tag.String())
	return nil
}

// annotate reads in the layers of the given v1.Image and annotates the xpkg
// layers with their corresponding annotations, returning a new v1.Image
// containing the annotation details.
func annotate(i v1.Image) (v1.Image, error) { //nolint:gocyclo
	cfgFile, err := i.ConfigFile()
	if err != nil {
		return nil, err
	}

	layers, err := i.Layers()
	if err != nil {
		return nil, err
	}

	addendums := make([]mutate.Addendum, 0)

	for _, l := range layers {
		d, err := l.Digest()
		if err != nil {
			return nil, err
		}
		if annotation, ok := cfgFile.Config.Labels[xpkg.Label(d.String())]; ok {
			addendums = append(addendums, mutate.Addendum{
				Layer: l,
				Annotations: map[string]string{
					xpkg.AnnotationKey: annotation,
				},
			})
			continue
		}
		addendums = append(addendums, mutate.Addendum{
			Layer: l,
		})
	}

	// we didn't find any annotations, return original image
	if len(addendums) == 0 {
		return i, nil
	}

	img := empty.Image
	for _, a := range addendums {
		img, err = mutate.Append(img, a)
		if err != nil {
			return nil, errors.Wrap(err, errBuildImage)
		}
	}

	return mutate.ConfigFile(img, cfgFile)
}
