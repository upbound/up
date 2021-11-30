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

package xpls

import (
	"os"
	"testing"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/test"
	xpextv1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
	xpextv1beta1 "github.com/crossplane/crossplane/apis/apiextensions/v1beta1"
	"github.com/google/go-cmp/cmp"
	"github.com/spf13/afero"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var (
	testComposition      []byte
	testInvalidXRD       []byte
	testMultipleObject   []byte
	testMultiVersionCRD  []byte
	testSingleVersionCRD []byte
)

func init() {
	testComposition, _ = afero.ReadFile(afero.NewOsFs(), "testdata/composition.yaml")
	testInvalidXRD, _ = afero.ReadFile(afero.NewOsFs(), "testdata/invalid-xrd.yaml")
	testMultipleObject, _ = afero.ReadFile(afero.NewOsFs(), "testdata/multiple-object.yaml")
	testMultiVersionCRD, _ = afero.ReadFile(afero.NewOsFs(), "testdata/multiple-version-crd.yaml")
	testSingleVersionCRD, _ = afero.ReadFile(afero.NewOsFs(), "testdata/single-version-crd.yaml")
}

func TestParse(t *testing.T) {
	cases := map[string]struct {
		reason string
		ws     *Workspace
		nodes  map[NodeIdentifier]struct{}
		err    error
	}{
		"ErrorRootNotExist": {
			reason: "Should return an error if the workspace root does not exist.",
			ws:     NewWorkspace("/ws", WithFS(afero.NewMemMapFs())),
			err:    &os.PathError{Op: "open", Path: "/ws", Err: afero.ErrFileNotFound},
		},
		"SuccessfulEmpty": {
			reason: "Should not return an error if the workspace is empty.",
			ws: NewWorkspace("/ws", WithFS(func() afero.Fs {
				fs := afero.NewMemMapFs()
				_ = fs.Mkdir("/ws", os.ModePerm)
				return fs
			}())),
		},
		"SuccessfulNoKubernetesObjects": {
			// NOTE(hasheddan): this test reflects current behavior, but we
			// should be surfacing errors / diagnostics if we fail to parse
			// objects in a package unless they are specified to be ignored. We
			// likely also want skip any non-YAML files by default as we do in
			// package parsing.
			reason: "Should have no package nodes if no Kubernetes objects are present.",
			ws: NewWorkspace("/ws", WithFS(func() afero.Fs {
				fs := afero.NewMemMapFs()
				_ = afero.WriteFile(fs, "/ws/somerandom.yaml", []byte("some invalid ::: yaml"), os.ModePerm)
				return fs
			}())),
		},
		"SuccessfulParseComposition": {
			reason: "Should add a package node for Composition and every embedded resource.",
			ws: NewWorkspace("/ws", WithFS(func() afero.Fs {
				fs := afero.NewMemMapFs()
				_ = afero.WriteFile(fs, "/ws/composition.yaml", testComposition, os.ModePerm)
				return fs
			}())),
			nodes: map[NodeIdentifier]struct{}{
				nodeID("", schema.FromAPIVersionAndKind("ec2.aws.crossplane.io/v1beta1", "VPC")):               {},
				nodeID("", schema.FromAPIVersionAndKind("ec2.aws.crossplane.io/v1beta1", "Subnet")):            {},
				nodeID("vpcpostgresqlinstances.aws.database.example.org", xpextv1.CompositionGroupVersionKind): {},
			},
		},
		"SuccessfulParseMultipleSameFile": {
			reason: "Should add a package node for every resource when multiple objects exist in single file.",
			ws: NewWorkspace("/ws", WithFS(func() afero.Fs {
				fs := afero.NewMemMapFs()
				_ = afero.WriteFile(fs, "/ws/multiple.yaml", testMultipleObject, os.ModePerm)
				return fs
			}())),
			nodes: map[NodeIdentifier]struct{}{
				nodeID("compositepostgresqlinstances.database.example.org", xpextv1.CompositeResourceDefinitionGroupVersionKind): {},
				nodeID("some.other.xrd", xpextv1.CompositeResourceDefinitionGroupVersionKind):                                    {},
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			if diff := cmp.Diff(tc.err, tc.ws.Parse(), test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nParse(...): -want error, +got error:\n%s", tc.reason, diff)
			}
			if len(tc.nodes) != len(tc.ws.nodes) {
				t.Errorf("\n%s\nParse(...): -want node count: %d, +got node count: %d", tc.reason, len(tc.nodes), len(tc.ws.nodes))
			}
			for id := range tc.ws.nodes {
				if _, ok := tc.nodes[id]; !ok {
					t.Errorf("\n%s\nParse(...): missing node:\n%v", tc.reason, id)
				}
			}
		})
	}
}

func TestLoadValidators(t *testing.T) {
	cases := map[string]struct {
		reason     string
		ws         *Workspace
		path       string
		validators map[schema.GroupVersionKind]struct{}
		err        error
	}{
		"ErrorPathNotExist": {
			reason: "Should return an error if the path does not exist.",
			ws:     NewWorkspace("/", WithFS(afero.NewMemMapFs())),
			path:   "/cache",
			err:    &os.PathError{Op: "open", Path: "/cache", Err: afero.ErrFileNotFound},
		},
		"SuccessfulNoKubernetesObjects": {
			reason: "Should return an error if the path does not exist.",
			ws: NewWorkspace("/ws", WithFS(func() afero.Fs {
				fs := afero.NewMemMapFs()
				_ = fs.Mkdir("/ws", os.ModePerm)
				_ = afero.WriteFile(fs, "/cache/somerandom.yaml", []byte("some invalid ::: yaml"), os.ModePerm)
				return fs
			}())),
			path: "/cache",
		},
		"SuccessfulLoadFromCRD": {
			reason: "Should add a validator for a CRD if it is valid.",
			ws: NewWorkspace("/ws", WithFS(func() afero.Fs {
				fs := afero.NewMemMapFs()
				_ = fs.Mkdir("/ws", os.ModePerm)
				_ = afero.WriteFile(fs, "/cache/valid.yaml", testSingleVersionCRD, os.ModePerm)
				return fs
			}())),
			path: "/cache",
			validators: map[schema.GroupVersionKind]struct{}{
				schema.FromAPIVersionAndKind("acm.aws.crossplane.io/v1alpha1", "Certificate"): {},
			},
		},
		"SuccessfulLoadMultiVersionFromCRD": {
			reason: "Should add a validator for each version in a CRD if multiple are specified.",
			ws: NewWorkspace("/ws", WithFS(func() afero.Fs {
				fs := afero.NewMemMapFs()
				_ = fs.Mkdir("/ws", os.ModePerm)
				_ = afero.WriteFile(fs, "/cache/multiversion.yaml", testMultiVersionCRD, os.ModePerm)
				return fs
			}())),
			path: "/cache",
			validators: map[schema.GroupVersionKind]struct{}{
				xpextv1.CompositeResourceDefinitionGroupVersionKind:      {},
				xpextv1beta1.CompositeResourceDefinitionGroupVersionKind: {},
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			if diff := cmp.Diff(tc.err, tc.ws.LoadValidators(tc.path), test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nLoadValidators(...): -want error, +got error:\n%s", tc.reason, diff)
			}
			if len(tc.validators) != len(tc.ws.validators) {
				t.Errorf("\n%s\nLoadValidators(...): -want validators count: %d, +got validators count: %d", tc.reason, len(tc.validators), len(tc.ws.validators))
			}
			for id := range tc.ws.validators {
				if _, ok := tc.validators[id]; !ok {
					t.Errorf("\n%s\nLoadValidators(...): missing validator:\n%v", tc.reason, id)
				}
			}
		})
	}
}

func TestValidate(t *testing.T) {
	cases := map[string]struct {
		reason string
		ws     *Workspace
		filter NodeFilterFn
		dsCnt  int
		err    error
	}{
		"ErrorMissingValidator": {
			reason: "Should return an error if we can't find a validator for the object kind.",
			ws: NewWorkspace("/ws", WithFS(func() afero.Fs {
				fs := afero.NewMemMapFs()
				_ = afero.WriteFile(fs, "/ws/xrd.yaml", testInvalidXRD, os.ModePerm)
				_ = afero.WriteFile(fs, "/cache/crd.yaml", testSingleVersionCRD, os.ModePerm)
				return fs
			}())),
			filter: AllNodes,
			err:    errors.Errorf(errMissingValidatorFmt, xpextv1.CompositeResourceDefinitionGroupVersionKind),
		},
		"SuccessfulNoNodes": {
			reason: "Should not return an error if no nodes match filter.",
			ws: NewWorkspace("/ws", WithFS(func() afero.Fs {
				fs := afero.NewMemMapFs()
				_ = afero.WriteFile(fs, "/ws/xrd.yaml", testInvalidXRD, os.ModePerm)
				_ = afero.WriteFile(fs, "/cache/crd.yaml", testSingleVersionCRD, os.ModePerm)
				return fs
			}())),
			filter: func(map[NodeIdentifier]Node) []Node { return nil },
		},
		"SuccessfulInvalidObject": {
			reason: "Should return a single diagnostic if we successfully validate and find a single error.",
			ws: NewWorkspace("/ws", WithFS(func() afero.Fs {
				fs := afero.NewMemMapFs()
				_ = afero.WriteFile(fs, "/ws/xrd.yaml", testInvalidXRD, os.ModePerm)
				_ = afero.WriteFile(fs, "/cache/crd.yaml", testMultiVersionCRD, os.ModePerm)
				return fs
			}())),
			filter: AllNodes,
			dsCnt:  1,
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			// TODO(hasheddan): consider pre-building validators and nodes so
			// that we aren't exercising Parse and LoadValidators when we just
			// want to test Validate.
			if err := tc.ws.Parse(); err != nil {
				t.Errorf("\n%s\nParse(...): unexpected error:\n%s", tc.reason, err)
			}
			if err := tc.ws.LoadValidators("/cache"); err != nil {
				t.Errorf("\n%s\nLoadValidators(...): unexpected error:\n%s", tc.reason, err)
			}
			ds, err := tc.ws.Validate(tc.filter)
			if diff := cmp.Diff(tc.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nValidate(...): -want error, +got error:\n%s", tc.reason, diff)
			}
			// TODO(hasheddan): we should build out a suite of tests for any
			// validation that we build custom functionality for, such as
			// warnings we might surface if an object is valid but not likely to
			// work.
			if len(ds) != tc.dsCnt {
				t.Errorf("\n%s\nValidate(...): -want diagnostics count: %d, +got diagnostics count: %d", tc.reason, tc.dsCnt, len(ds))
			}
		})
	}
}
