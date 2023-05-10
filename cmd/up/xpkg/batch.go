// Copyright 2023 Upbound Inc
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
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"text/template"

	"github.com/alecthomas/kong"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/mutate"

	"github.com/upbound/up/internal/upbound"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/parser"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/pterm/pterm"
	"github.com/spf13/afero"

	"github.com/upbound/up/internal/xpkg"
	"github.com/upbound/up/internal/xpkg/parser/examples"
	"github.com/upbound/up/internal/xpkg/parser/yaml"
)

const (
	errInvalidTemplate    = "service-scoped provider template is not valid"
	errMetadataBackend    = "failed to initialize the package metadata parser backend"
	errCRDBackend         = "failed to initialize the CRD parser backend"
	errTemplateFmt        = "failed to execute the provider metadata template using: %v"
	errInvalidPlatformFmt = "failed to parse platform name. Expected syntax is <OS>_<arch>: %s"
	errBuildPackageFmt    = "failed to service-scoped provider package: %s"
	errGetConfigFmt       = "failed to get config file from %s image for service %q"
	errMutateConfigFmt    = "failed to mutate config file from %s image for service %q"
	errGetLayersFmt       = "failed to get layers from %s image for service %q"
	errGetBaseLayersFmt   = "failed to get base layers from %s image for service %q"
	errGetDigestFmt       = "failed to get layer's digest from %s image for service %q"
	errAppendLayersFmt    = "failed to append layers to %s image for service %q"
	errReadProviderBinFmt = "failed to read %q provider binary for %s platform from path: %s"
	errNewLayerFmt        = "failed to initialize a new image layer for %s platform for service %q"
	errAddLayerFmt        = "failed to add the service-scoped provider binary layer for %s platform for service %q"
	errPushPackageFmt     = "failed to push service-scoped provider package: %s"
	errReadAuthExtFmt     = "failed to read the authentication extension file at: %s"
)

const (
	wildcard = "*"
)

// AfterApply constructs and binds Upbound-specific context to any subcommands
// that have Run() methods that receive it.
func (c *batchCmd) AfterApply(kongCtx *kong.Context) error {
	c.fs = afero.NewOsFs()
	// NOTE(aru): we currently only support fetching family base image from
	// daemon, but may opt to support additional sources in the future.
	c.fetch = daemonFetch
	c.wg = &sync.WaitGroup{}

	upCtx, err := upbound.NewFromFlags(c.Flags)
	if err != nil {
		return err
	}
	kongCtx.Bind(upCtx)

	return nil
}

// batchCmd builds and pushes a family of Crossplane provider packages.
type batchCmd struct {
	fs    afero.Fs
	fetch fetchFn

	FamilyBaseImage        string   `help:"Family image used as the base for the service-scoped provider packages." required:""`
	ProviderName           string   `help:"Provider name, such as provider-aws to be used while formatting service-scoped provider package repositories." required:""`
	FamilyPackageURLFormat string   `help:"Family package URL format to be used for the service-scoped provider packages. Must be a valid OCI image URL with the format specifier \"%s\", which will be substituted with <provider name>-<service name>." required:""`
	Service                []string `help:"Services to build the scoped provider packages for." default:"monolith"`
	Concurrency            uint     `help:"Maximum number of packages to process concurrently. Setting it to 0 puts no limit on the concurrency, i.e., all packages are processed in parallel." default:"0"`

	Platform        []string `help:"Platforms to build the packages for. Each platform should use the <OS>_<arch> syntax. An example is: linux_arm64." default:"linux_amd64,linux_arm64"`
	ProviderBinRoot string   `short:"p" help:"Provider binary paths root. Service-scoped provider binaries should reside under the platform directories in this folder." type:"existingdir"`

	PackageMetadataTemplate string            `help:"Service-scoped provider metadata template. The template variables {{ .Service }} and {{ .Name }} will be substituted when the template is executed among with the supplied template variable substitutions." default:"./package/crossplane.yaml.tmpl" type:"path"`
	TemplateVar             map[string]string `help:"Service-scoped provider metadata template variables to be used for the specified template."`

	ExamplesGroupOverride map[string]string `help:"Overrides for the location of the example manifests folder of a service-scoped provider." optional:""`
	CRDGroupOverride      map[string]string `help:"Overrides for the locations of the CRD folders of the service-scoped providers." optional:""`
	PackageRepoOverride   map[string]string `help:"Overrides for the package repository names of the service-scoped providers." optional:""`
	AuthExtService        []string          `help:"Service name to add the authentication extension for." default:"monolith,config"`

	ExamplesRoot string   `short:"e" help:"Path to package examples directory." default:"./examples" type:"path"`
	CRDRoot      string   `help:"Path to package CRDs directory." default:"./package/crds" type:"path"`
	AuthExt      string   `help:"Path to an authentication extension file." default:"./package/auth.yaml" type:"path"`
	Ignore       []string `help:"Paths to exclude from the service-scoped provider packages."`
	Create       bool     `help:"Create repository on push if it does not exist."`

	// Common Upbound API configuration
	Flags upbound.Flags `embed:""`

	wg *sync.WaitGroup
}

// Run executes the batch command.
func (c *batchCmd) Run(p pterm.TextPrinter, upCtx *upbound.Context) error { //nolint:gocyclo
	baseImgMap := make(map[string]v1.Image, len(c.Platform))
	for _, p := range c.Platform {
		tokens := strings.Split(p, "_")
		if len(tokens) != 2 {
			return errors.Errorf(errInvalidPlatformFmt, p)
		}
		ref, err := name.ParseReference(fmt.Sprintf("%s-%s", c.FamilyBaseImage, tokens[1]))
		if err != nil {
			return err
		}
		img, err := c.fetch(context.Background(), ref)
		if err != nil {
			return err
		}
		baseImgMap[p] = img // assumes correct OS
	}

	concurrency := make(chan struct{}, c.Concurrency)
	for i := uint(0); i < c.Concurrency; i++ {
		concurrency <- struct{}{}
	}
	for _, s := range c.Service {
		c.wg.Add(1)
		s := s
		go func() {
			defer c.wg.Done()
			// if concurrency is limited
			if c.Concurrency != 0 {
				<-concurrency
				defer func() {
					concurrency <- struct{}{}
				}()
			}
			if err := c.processService(p, upCtx, baseImgMap, s); err != nil {
				p.PrintOnErrorf("Publishing of service-scoped provider package has failed for service %q: %v", s, err)
			}
		}()
	}
	c.wg.Wait()
	close(concurrency)
	return nil
}

func (c *batchCmd) processService(p pterm.TextPrinter, upCtx *upbound.Context, baseImgMap map[string]v1.Image, s string) error { //nolint:gocyclo
	imgs := make([]v1.Image, 0, len(c.Platform))
	var addendumLayers []v1.Layer
	var labels [][2]string
	for _, p := range c.Platform {
		var img v1.Image
		var err error
		switch {
		case len(addendumLayers) > 0:
			img = baseImgMap[p]
			for _, l := range addendumLayers {
				img, err = mutate.AppendLayers(img, l)
				if err != nil {
					return errors.Wrapf(err, errAppendLayersFmt, p, s)
				}
			}
			cfg, err := img.ConfigFile()
			if err != nil {
				return errors.Wrapf(err, errGetConfigFmt, p, s)
			}
			if cfg.Config.Labels == nil {
				cfg.Config.Labels = make(map[string]string, len(labels))
			}
			for _, kv := range labels {
				if kv[1] == "" {
					continue
				}
				cfg.Config.Labels[kv[0]] = kv[1]
			}
			img, err = mutate.Config(img, cfg.Config)
			if err != nil {
				return errors.Wrapf(err, errMutateConfigFmt, p, s)
			}
			img, err = c.addProviderBinaryLayer(img, p, s)
			if err != nil {
				return err
			}
		default:
			img, err = c.buildImage(baseImgMap, p, s)
			if err != nil {
				return err
			}
			// calculate addendum layers to reuse
			addendumLayers, labels, err = getAddendumLayers(baseImgMap[p], img, p, s)
			if err != nil {
				return err
			}
		}
		imgs = append(imgs, img)
	}

	t := c.getPackageURL(s)
	p.Printfln("Pushing xpkg to %s", t)
	if err := PushImages(p, upCtx, imgs, t, c.Create, c.Flags.Profile); err != nil {
		return errors.Wrapf(err, errPushPackageFmt, s)
	}
	return nil
}

func (c *batchCmd) getPackageRepo(s string) string {
	repo := c.PackageRepoOverride[s]
	if repo == "" {
		repo = fmt.Sprintf("%s-%s", c.ProviderName, s)
	}
	return repo
}

func (c *batchCmd) getPackageURL(s string) string {
	return fmt.Sprintf(c.FamilyPackageURLFormat, c.getPackageRepo(s))
}

func getAddendumLayers(baseImg, img v1.Image, p, s string) ([]v1.Layer, [][2]string, error) {
	baseLayers, err := baseImg.Layers()
	if err != nil {
		return nil, nil, errors.Wrapf(err, errGetBaseLayersFmt, p, s)
	}
	layers, err := img.Layers()
	if err != nil {
		return nil, nil, errors.Wrapf(err, errGetLayersFmt, p, s)
	}
	addendumLayers := layers[len(baseLayers) : len(layers)-1]
	// get associated labels from image config
	cfg, err := img.ConfigFile()
	if err != nil {
		return nil, nil, errors.Wrapf(err, errGetConfigFmt, p, s)
	}
	labels := make([][2]string, 0, len(addendumLayers))
	for _, l := range addendumLayers {
		d, err := l.Digest()
		if err != nil {
			return nil, nil, errors.Wrapf(err, errGetDigestFmt, p, s)
		}
		label := ""
		key := xpkg.Label(d.String())
		for k, v := range cfg.Config.Labels {
			if key == k {
				label = v
				break
			}
		}
		labels = append(labels, [2]string{key, label})
	}
	return addendumLayers, labels, nil
}

func (c *batchCmd) buildImage(baseImgMap map[string]v1.Image, p, s string) (v1.Image, error) {
	builder, err := c.getBuilder(s)
	if err != nil {
		return nil, err
	}
	img, _, err := builder.Build(context.Background(), xpkg.WithController(baseImgMap[p]))
	if err != nil {
		return nil, errors.Wrapf(err, errBuildPackageFmt, s)
	}
	return c.addProviderBinaryLayer(img, p, s)
}

func (c *batchCmd) addProviderBinaryLayer(img v1.Image, p, s string) (v1.Image, error) {
	configFile, err := img.ConfigFile()
	if err != nil {
		return nil, errors.Wrapf(err, errGetConfigFmt, p, s)
	}
	binPath := filepath.Join(c.ProviderBinRoot, p, s)
	buff, err := os.ReadFile(filepath.Clean(binPath))
	if err != nil {
		return nil, errors.Wrapf(err, errReadProviderBinFmt, s, p, binPath)
	}
	l, err := xpkg.Layer(bytes.NewBuffer(buff), "/usr/local/bin/provider", "", int64(len(buff)), 0o755, &configFile.Config)
	if err != nil {
		return nil, errors.Wrapf(err, errNewLayerFmt, p, s)
	}
	img, err = mutate.AppendLayers(img, l)
	return img, errors.Wrapf(err, errAddLayerFmt, p, s)
}

func (c *batchCmd) getExamplesGroup(service string) string {
	p := c.ExamplesGroupOverride[service]
	if p == wildcard {
		p = ""
	} else if p == "" {
		p = service
	}
	return filepath.Join(c.ExamplesRoot, p)
}

func (c *batchCmd) getAuthBackend(ax string) (parser.Backend, error) {
	axf, err := c.fs.Open(ax)
	// configuring an auth extension is optional as before
	if err != nil {
		return nil, nil
	}

	defer func() { _ = axf.Close() }()
	b, err := io.ReadAll(axf)
	if err != nil {
		return nil, errors.Wrapf(err, errReadAuthExtFmt, ax)
	}
	return parser.NewEchoBackend(string(b)), nil
}

func (c *batchCmd) getBuilder(service string) (*xpkg.Builder, error) {
	ex, err := filepath.Abs(c.getExamplesGroup(service))
	if err != nil {
		return nil, err
	}

	var authBE parser.Backend
	if ax, err := filepath.Abs(c.AuthExt); err == nil {
		for _, s := range c.AuthExtService {
			if service != s {
				continue
			}
			authBE, err = c.getAuthBackend(ax)
			if err != nil {
				return nil, err
			}
			break
		}
	}

	pp, err := yaml.New()
	if err != nil {
		return nil, err
	}

	packageMetadata, err := c.getPackageMetadata(service)
	if err != nil {
		return nil, err
	}

	return xpkg.New(
		&batchParserBackend{
			packageMetadata: packageMetadata,
			service:         service,
			crdsRoot:        c.CRDRoot,
			fs:              c.fs,
			options: []parser.BackendOption{
				parser.FsDir(c.CRDRoot),
				parser.FsFilters(
					append(
						buildFilters(c.CRDRoot, c.Ignore),
						xpkg.SkipContains(c.ExamplesRoot), xpkg.SkipContains(c.AuthExt),
						func(_ string, info os.FileInfo) (bool, error) {
							return !strings.HasPrefix(info.Name(), c.getCRDPrefix(service)), nil
						})...),
			},
		},
		authBE,
		parser.NewFsBackend(
			c.fs,
			parser.FsDir(ex),
			parser.FsFilters(
				buildFilters(ex, c.Ignore)...),
		),
		pp,
		examples.New(),
	), nil
}

func (c *batchCmd) getCRDPrefix(service string) string {
	o := c.CRDGroupOverride[service]
	if o == wildcard {
		return ""
	}
	if o == "" {
		o = service
	}
	return o + "."
}

func (c *batchCmd) getPackageMetadata(service string) (string, error) {
	tmpl, err := template.New(filepath.Base(c.PackageMetadataTemplate)).ParseFiles(c.PackageMetadataTemplate)
	if err != nil {
		return "", errors.Wrap(err, errInvalidTemplate)
	}

	// prepare template var substitutions
	data := make(map[string]string, len(c.TemplateVar)+2)
	data["Service"] = service
	data["Name"] = c.getPackageRepo(service)
	// copy substitutions passed from the command-line
	for k, v := range c.TemplateVar {
		data[k] = v
	}

	buff := &bytes.Buffer{}
	err = tmpl.Execute(buff, data)
	if err != nil {
		return "", errors.Wrapf(err, errTemplateFmt, data)
	}
	return buff.String(), nil
}

type batchParserBackend struct {
	packageMetadata string
	service         string
	crdsRoot        string
	options         []parser.BackendOption

	fs afero.Fs
}

func (b *batchParserBackend) Init(ctx context.Context, opts ...parser.BackendOption) (io.ReadCloser, error) {
	rcMetadata, err := parser.NewEchoBackend(b.packageMetadata).Init(ctx, opts...)
	if err != nil {
		return nil, errors.Wrap(err, errMetadataBackend)
	}
	rcCRD, err := parser.NewFsBackend(b.fs, b.options...).Init(ctx, opts...)
	if err != nil {
		return nil, errors.Wrap(err, errCRDBackend)
	}
	return &batchReadCloser{
		metadataReadCloser: rcMetadata,
		crdReadCloser:      rcCRD,
	}, nil
}

type batchReadCloser struct {
	metadataReadCloser io.ReadCloser
	crdReadCloser      io.ReadCloser
	metadataRead       bool
}

func (b *batchReadCloser) Read(p []byte) (n int, err error) {
	if !b.metadataRead {
		b.metadataRead = true
		return b.metadataReadCloser.Read(p)
	}
	return b.crdReadCloser.Read(p)
}

func (b *batchReadCloser) Close() error {
	return b.crdReadCloser.Close() // echo backend's io.Closer implementation is a noop one.
}
