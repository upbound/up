// Copyright 2024 Upbound Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package build

import (
	"context"
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/parser"
	xpv1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
	metav1 "github.com/crossplane/crossplane/apis/pkg/meta/v1"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
	"github.com/pterm/pterm"
	"github.com/spf13/afero"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/yaml"

	"github.com/upbound/up/internal/xpkg"
	"github.com/upbound/up/internal/xpkg/parser/examples"
	pyaml "github.com/upbound/up/internal/xpkg/parser/yaml"

	"github.com/upbound/up/pkg/apis/project/v1alpha1"
)

type Cmd struct {
	ProjectFile string `short:"f" help:"Path to project definition file." default:"upbound.yaml"`
	Repository  string `optional:"" help:"Repository for the built package. Overrides the repository specified in the project file."`
	Tag         string `short:"t" help:"Tag for the built package." default:"latest"`
	OutputDir   string `short:"o" help:"Path to the output directory, where packages will be written." default:"_output"`

	projFS   afero.Fs
	outputFS afero.Fs
}

func (c *Cmd) AfterApply() error {
	// Read the project file.
	projFilePath, err := filepath.Abs(c.ProjectFile)
	if err != nil {
		return err
	}
	// The location of the project file defines the root of the project.
	projDirPath := filepath.Dir(projFilePath)
	// Construct a virtual filesystem that contains only the project. We'll do
	// all our operations inside this virtual FS.
	c.projFS = afero.NewBasePathFs(afero.NewOsFs(), projDirPath)

	// Output can be anywhere, doesn't have to be in the project directory.
	c.outputFS = afero.NewOsFs()

	return nil
}

func (c *Cmd) Run(ctx context.Context, p pterm.TextPrinter) error { //nolint:gocyclo // This is fine.
	// Parse and validate the project file.
	projYAML, err := afero.ReadFile(c.projFS, filepath.Join("/", filepath.Base(c.ProjectFile)))
	if err != nil {
		return errors.Wrapf(err, "failed to read project file %q", c.ProjectFile)
	}
	var project v1alpha1.Project
	err = yaml.Unmarshal(projYAML, &project)
	if err != nil {
		return errors.Wrap(err, "failed to parse project file")
	}
	if err := project.Validate(); err != nil {
		return errors.Wrap(err, "invalid project file")
	}

	if c.Repository != "" {
		project.Spec.Repository = c.Repository
	}

	// Construct absolute versions of the other configured paths for use within
	// the virtual FS.
	apisPath := "/"
	if project.Spec.Paths != nil && project.Spec.Paths.APIs != "" {
		apisPath = filepath.Clean(filepath.Join("/", project.Spec.Paths.APIs))
	}
	examplesPath := "/examples"
	if project.Spec.Paths != nil && project.Spec.Paths.Examples != "" {
		examplesPath = filepath.Clean(filepath.Join("/", project.Spec.Paths.Examples))
	}

	// Scaffold a configuration based on the metadata in the project. Later
	// we'll add any embedded functions we build to the depednencies.
	cfg := &metav1.Configuration{
		TypeMeta: v1.TypeMeta{
			APIVersion: metav1.SchemeGroupVersion.String(),
			Kind:       metav1.ConfigurationKind,
		},
		ObjectMeta: cfgMetaFromProject(&project),
		Spec: metav1.ConfigurationSpec{
			MetaSpec: metav1.MetaSpec{
				Crossplane: project.Spec.Crossplane,
				DependsOn:  project.Spec.DependsOn,
			},
		},
	}

	// Collect APIs (composites). By default we search the whole project
	// directory except the examples directory.
	apisSource := c.projFS
	apiExcludes := []string{examplesPath}
	if apisPath != "/" {
		apisSource = afero.NewBasePathFs(c.projFS, apisPath)
		apiExcludes = []string{}
	}
	packageFS, err := collectComposites(apisSource, apiExcludes)
	if err != nil {
		return err
	}

	// Add the package metadata.
	y, err := yaml.Marshal(cfg)
	if err != nil {
		return errors.Wrap(err, "failed to marshal package metadata")
	}
	err = afero.WriteFile(packageFS, "/crossplane.yaml", y, 0644)
	if err != nil {
		return errors.Wrap(err, "failed to write package metadata")
	}

	pp, err := pyaml.New()
	if err != nil {
		return errors.Wrap(err, "failed to create parser")
	}
	builder := xpkg.New(
		parser.NewFsBackend(packageFS, parser.FsDir("/")),
		nil,
		parser.NewFsBackend(afero.NewBasePathFs(c.projFS, examplesPath),
			parser.FsDir("/"),
			parser.FsFilters(parser.SkipNotYAML()),
		),
		pp,
		examples.New(),
	)

	img, _, err := builder.Build(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to build package")
	}

	pkgName := fmt.Sprintf("%s-%s", project.Name, c.Tag)
	outFile := xpkg.BuildPath(c.OutputDir, pkgName)

	err = c.outputFS.MkdirAll(c.OutputDir, 0755)
	if err != nil {
		return errors.Wrapf(err, "failed to create output directory %q", c.OutputDir)
	}

	f, err := c.outputFS.Create(outFile)
	if err != nil {
		return errors.Wrapf(err, "failed to create output file %q", outFile)
	}
	defer f.Close() //nolint:errcheck // Can't do anything useful with this error.

	err = tarball.Write(nil, img, f)
	if err != nil {
		return errors.Wrap(err, "failed to write package to file")
	}
	p.Printfln("Wrote package to file %s", outFile)

	return nil
}

func collectComposites(fromFS afero.Fs, exclude []string) (afero.Fs, error) { //nolint:gocyclo // This is fine.
	toFS := afero.NewMemMapFs()
	return toFS, afero.Walk(fromFS, "/", func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			return err
		}

		for _, excl := range exclude {
			if strings.HasPrefix(path, excl) {
				return filepath.SkipDir
			}
		}

		if info.IsDir() {
			return nil
		}
		// Ignore files without yaml extensions.
		ext := filepath.Ext(path)
		if ext != ".yaml" && ext != ".yml" {
			return nil
		}

		var u unstructured.Unstructured
		bs, err := afero.ReadFile(fromFS, path)
		if err != nil {
			return errors.Wrapf(err, "failed to read file %q", path)
		}
		err = yaml.Unmarshal(bs, &u)
		if err != nil {
			return errors.Wrapf(err, "failed to parse file %q", path)
		}

		// Ignore anything that's not an XRD or Composition, since those are the
		// only allowed types in a Configuration xpkg.
		if u.GroupVersionKind().Group != xpv1.Group {
			return nil
		}
		if u.GetKind() != xpv1.CompositeResourceDefinitionKind && u.GetKind() != xpv1.CompositionKind {
			return nil
		}

		// Copy the file into the package FS.
		err = afero.WriteFile(toFS, path, bs, 0644)
		if err != nil {
			return errors.Wrapf(err, "failed to write file %q to package", path)
		}

		return nil
	})
}

func cfgMetaFromProject(proj *v1alpha1.Project) v1.ObjectMeta {
	meta := proj.ObjectMeta.DeepCopy()

	if meta.Annotations == nil {
		meta.Annotations = make(map[string]string)
	}

	meta.Annotations["meta.crossplane.io/maintainer"] = proj.Spec.Maintainer
	meta.Annotations["meta.crossplane.io/source"] = proj.Spec.Source
	meta.Annotations["meta.crossplane.io/license"] = proj.Spec.License
	meta.Annotations["meta.crossplane.io/description"] = proj.Spec.Description
	meta.Annotations["meta.crossplane.io/readme"] = proj.Spec.Readme

	return *meta
}
