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
	"embed"
	"io"
	"strings"
	"testing"

	metav1 "github.com/crossplane/crossplane/apis/pkg/meta/v1"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
	"github.com/pterm/pterm"
	"github.com/spf13/afero"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/yaml"

	"github.com/upbound/up/internal/xpkg"
	xpkgmarshaler "github.com/upbound/up/internal/xpkg/dep/marshaler/xpkg"
	"github.com/upbound/up/pkg/apis/project/v1alpha1"
)

//go:embed testdata/configuration-getting-started/**
var configurationGettingStarted embed.FS

func TestBuild(t *testing.T) {
	projFS := afero.NewBasePathFs(afero.FromIOFS{FS: configurationGettingStarted}, "testdata/configuration-getting-started")
	outFS := afero.NewMemMapFs()

	c := &Cmd{
		ProjectFile: "upbound.yaml",
		Tag:         "unittest",
		OutputDir:   "_output",

		projFS:   projFS,
		outputFS: outFS,
	}

	// Parse the upbound.yaml from the example so we can validate that certain
	// fields were copied correctly later in the test.
	var project v1alpha1.Project
	y, err := afero.ReadFile(projFS, "upbound.yaml")
	assert.NilError(t, err)
	err = yaml.Unmarshal(y, &project)
	assert.NilError(t, err)

	// Build the package.
	err = c.Run(context.Background(), &pterm.BasicTextPrinter{
		Style:  pterm.DefaultBasicText.Style,
		Writer: &TestWriter{t},
	})
	assert.NilError(t, err)

	// Validate the package:
	// 1. Extract it to make sure it's a valid OCI image tarball.
	// 2. Unmarshal it to make sure it's a valid Crossplane package.
	// 3. Lint it to make sure it's a valid Crossplane Configuration package.
	// 4. Check that the package metadata is correctly constructed.
	// 5. Check that the package has the right number of objects in it.
	img, err := tarball.Image(func() (io.ReadCloser, error) {
		return outFS.Open("_output/configuration-getting-started-unittest.xpkg")
	}, nil)
	assert.NilError(t, err)

	m, err := xpkgmarshaler.NewMarshaler()
	assert.NilError(t, err)
	pkg, err := m.FromImage(xpkg.Image{
		Image: img,
	})
	assert.NilError(t, err)

	linter := xpkg.NewConfigurationLinter()
	err = linter.Lint(&PackageAdapter{pkg})
	assert.NilError(t, err)

	meta := pkg.Meta()
	assert.DeepEqual(t, meta, &metav1.Configuration{
		TypeMeta: v1.TypeMeta{
			APIVersion: metav1.SchemeGroupVersion.String(),
			Kind:       metav1.ConfigurationKind,
		},
		ObjectMeta: v1.ObjectMeta{
			Name: project.Name,
			Annotations: map[string]string{
				"meta.crossplane.io/maintainer":  project.Spec.Maintainer,
				"meta.crossplane.io/source":      project.Spec.Source,
				"meta.crossplane.io/license":     project.Spec.License,
				"meta.crossplane.io/description": project.Spec.Description,
				"meta.crossplane.io/readme":      project.Spec.Readme,
			},
		},
		Spec: metav1.ConfigurationSpec{
			MetaSpec: metav1.MetaSpec{
				Crossplane: project.Spec.Crossplane,
				DependsOn:  project.Spec.DependsOn,
			},
		},
	})

	objs := pkg.Objects()
	// TODO(adamwg): There are 8 APIs, which should mean 16 objects - 8 XRDs and
	// 8 compositions. But right now we generate CRDs during parsing and inject
	// them into the package as well, which doubles the count. This assertion
	// will need to change when we refactor the dependency manager to generate
	// the CRDs after, rather than during, package loading.
	assert.Assert(t, cmp.Len(objs, 32))
}

type TestWriter struct {
	t *testing.T
}

func (w *TestWriter) Write(b []byte) (int, error) {
	out := strings.TrimRight(string(b), "\n")
	w.t.Log(out)
	return len(b), nil
}

// PackageAdapter translates a `ParsedPackage` from the xpkg marshaler into a
// `linter.Package` so we can lint it.
type PackageAdapter struct {
	wrap *xpkgmarshaler.ParsedPackage
}

func (pkg *PackageAdapter) GetMeta() []runtime.Object {
	return []runtime.Object{pkg.wrap.Meta()}
}

func (pkg *PackageAdapter) GetObjects() []runtime.Object {
	return pkg.wrap.Objects()
}
