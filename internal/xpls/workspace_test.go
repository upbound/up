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
	"context"
	"os"
	"testing"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/test"
	xpextv1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
	xpextv1beta1 "github.com/crossplane/crossplane/apis/apiextensions/v1beta1"
	"github.com/golang/tools/lsp/protocol"
	"github.com/golang/tools/span"
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
		opt    WorkspaceOpt
		nodes  map[NodeIdentifier]struct{}
		err    error
	}{
		"ErrorRootNotExist": {
			reason: "Should return an error if the workspace root does not exist.",
			opt:    WithFS(afero.NewMemMapFs()),
			err:    &os.PathError{Op: "open", Path: "/ws", Err: afero.ErrFileNotFound},
		},
		"SuccessfulEmpty": {
			reason: "Should not return an error if the workspace is empty.",
			opt: WithFS(func() afero.Fs {
				fs := afero.NewMemMapFs()
				_ = fs.Mkdir("/ws", os.ModePerm)
				return fs
			}()),
		},
		"SuccessfulNoKubernetesObjects": {
			// NOTE(hasheddan): this test reflects current behavior, but we
			// should be surfacing errors / diagnostics if we fail to parse
			// objects in a package unless they are specified to be ignored. We
			// likely also want skip any non-YAML files by default as we do in
			// package parsing.
			reason: "Should have no package nodes if no Kubernetes objects are present.",
			opt: WithFS(func() afero.Fs {
				fs := afero.NewMemMapFs()
				_ = afero.WriteFile(fs, "/ws/somerandom.yaml", []byte("some invalid ::: yaml"), os.ModePerm)
				return fs
			}()),
		},
		"SuccessfulParseComposition": {
			reason: "Should add a package node for Composition and every embedded resource.",
			opt: WithFS(func() afero.Fs {
				fs := afero.NewMemMapFs()
				_ = afero.WriteFile(fs, "/ws/composition.yaml", testComposition, os.ModePerm)
				return fs
			}()),
			nodes: map[NodeIdentifier]struct{}{
				nodeID("", schema.FromAPIVersionAndKind("ec2.aws.crossplane.io/v1beta1", "VPC")):               {},
				nodeID("", schema.FromAPIVersionAndKind("ec2.aws.crossplane.io/v1beta1", "Subnet")):            {},
				nodeID("vpcpostgresqlinstances.aws.database.example.org", xpextv1.CompositionGroupVersionKind): {},
			},
		},
		"SuccessfulParseMultipleSameFile": {
			reason: "Should add a package node for every resource when multiple objects exist in single file.",
			opt: WithFS(func() afero.Fs {
				fs := afero.NewMemMapFs()
				_ = afero.WriteFile(fs, "/ws/multiple.yaml", testMultipleObject, os.ModePerm)
				return fs
			}()),
			nodes: map[NodeIdentifier]struct{}{
				nodeID("compositepostgresqlinstances.database.example.org", xpextv1.CompositeResourceDefinitionGroupVersionKind): {},
				nodeID("some.other.xrd", xpextv1.CompositeResourceDefinitionGroupVersionKind):                                    {},
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			ws, _ := NewWorkspace("/ws", "/cache", tc.opt)

			if diff := cmp.Diff(tc.err, ws.Parse(), test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nParse(...): -want error, +got error:\n%s", tc.reason, diff)
			}
			if len(tc.nodes) != len(ws.nodes) {
				t.Errorf("\n%s\nParse(...): -want node count: %d, +got node count: %d", tc.reason, len(tc.nodes), len(ws.nodes))
			}
			for id := range ws.nodes {
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
		opt        WorkspaceOpt
		wsroot     string
		path       string
		validators map[schema.GroupVersionKind]struct{}
		err        error
	}{
		"ErrorPathNotExist": {
			reason: "Should return an error if the path does not exist.",
			wsroot: "/",
			opt:    WithFS(afero.NewMemMapFs()),
			path:   "/cache",
			err:    &os.PathError{Op: "open", Path: "/cache", Err: afero.ErrFileNotFound},
		},
		"SuccessfulNoKubernetesObjects": {
			reason: "Should return an error if the path does not exist.",
			wsroot: "/ws",
			opt: WithFS(func() afero.Fs {
				fs := afero.NewMemMapFs()
				_ = fs.Mkdir("/ws", os.ModePerm)
				_ = afero.WriteFile(fs, "/cache/somerandom.yaml", []byte("some invalid ::: yaml"), os.ModePerm)
				return fs
			}()),
			path: "/cache",
		},
		"SuccessfulLoadFromCRD": {
			reason: "Should add a validator for a CRD if it is valid.",
			wsroot: "/ws",
			opt: WithFS(func() afero.Fs {
				fs := afero.NewMemMapFs()
				_ = fs.Mkdir("/ws", os.ModePerm)
				_ = afero.WriteFile(fs, "/cache/valid.yaml", testSingleVersionCRD, os.ModePerm)
				return fs
			}()),
			path: "/cache",
			validators: map[schema.GroupVersionKind]struct{}{
				schema.FromAPIVersionAndKind("acm.aws.crossplane.io/v1alpha1", "Certificate"): {},
			},
		},
		"SuccessfulLoadMultiVersionFromCRD": {
			reason: "Should add a validator for each version in a CRD if multiple are specified.",
			wsroot: "/ws",
			opt: WithFS(func() afero.Fs {
				fs := afero.NewMemMapFs()
				_ = fs.Mkdir("/ws", os.ModePerm)
				_ = afero.WriteFile(fs, "/cache/multiversion.yaml", testMultiVersionCRD, os.ModePerm)
				return fs
			}()),
			path: "/cache",
			validators: map[schema.GroupVersionKind]struct{}{
				xpextv1.CompositeResourceDefinitionGroupVersionKind:      {},
				xpextv1beta1.CompositeResourceDefinitionGroupVersionKind: {},
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			ws, _ := NewWorkspace(tc.wsroot, "/cache", tc.opt)

			if diff := cmp.Diff(tc.err, ws.LoadValidators(tc.path), test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nLoadValidators(...): -want error, +got error:\n%s", tc.reason, diff)
			}
			if len(tc.validators) != len(ws.snapshot.validators) {
				t.Errorf("\n%s\nLoadValidators(...): -want validators count: %d, +got validators count: %d", tc.reason, len(tc.validators), len(ws.snapshot.validators))
			}
			for id := range ws.snapshot.validators {
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
		opt    WorkspaceOpt
		filter NodeFilterFn
		dsCnt  int
		err    error
	}{
		"SuccessfulNoNodes": {
			reason: "Should not return an error if no nodes match filter.",
			opt: WithFS(func() afero.Fs {
				fs := afero.NewMemMapFs()
				_ = afero.WriteFile(fs, "/ws/xrd.yaml", testInvalidXRD, os.ModePerm)
				_ = afero.WriteFile(fs, "/cache/crd.yaml", testSingleVersionCRD, os.ModePerm)
				return fs
			}()),
			filter: func(map[NodeIdentifier]Node) []Node { return nil },
		},
		"SuccessfulInvalidObject": {
			reason: "Should return a single diagnostic if we successfully validate and find a single error.",
			opt: WithFS(func() afero.Fs {
				fs := afero.NewMemMapFs()
				_ = afero.WriteFile(fs, "/ws/xrd.yaml", testInvalidXRD, os.ModePerm)
				_ = afero.WriteFile(fs, "/cache/crd.yaml", testMultiVersionCRD, os.ModePerm)
				return fs
			}()),
			filter: AllNodes,
			dsCnt:  1,
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			ws, _ := NewWorkspace("/ws", "/cache", tc.opt)
			// TODO(hasheddan): consider pre-building validators and nodes so
			// that we aren't exercising Parse and LoadValidators when we just
			// want to test Validate.
			if err := ws.Parse(); err != nil {
				t.Errorf("\n%s\nParse(...): unexpected error:\n%s", tc.reason, err)
			}
			if err := ws.LoadValidators("/cache"); err != nil {
				t.Errorf("\n%s\nLoadValidators(...): unexpected error:\n%s", tc.reason, err)
			}
			ds, err := ws.Validate(tc.filter)
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

func TestUpdateChanges(t *testing.T) {
	ctx := context.Background()

	type args struct {
		uri     span.URI
		changes []protocol.TextDocumentContentChangeEvent
		prebody []byte
	}
	type want struct {
		content []byte
		err     error
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"SuccessfullyInjectedChanges": {
			args: args{
				uri: span.URIFromPath("/ws/xrd.yaml"),
				changes: []protocol.TextDocumentContentChangeEvent{
					{
						Range: &protocol.Range{
							Start: protocol.Position{
								Line:      3,
								Character: 37,
							},
							End: protocol.Position{
								Line:      3,
								Character: 37,
							},
						},
						Text: "d",
					},
				},
				prebody: []byte(`apiVersion: apiextensions.crossplane.io/v1
kind: CompositeResourceDefinition
metadata:
  name: compositepostgresqlinstances.atabase.example.org`),
			},
			want: want{
				content: []byte(`apiVersion: apiextensions.crossplane.io/v1
kind: CompositeResourceDefinition
metadata:
  name: compositepostgresqlinstances.database.example.org`),
			},
		},
		"SuccessfullyInjectedMultipleCharacterChanges": {
			args: args{
				uri: span.URIFromPath("/ws/xrd.yaml"),
				changes: []protocol.TextDocumentContentChangeEvent{
					{
						Range: &protocol.Range{
							Start: protocol.Position{
								Line:      3,
								Character: 37,
							},
							End: protocol.Position{
								Line:      3,
								Character: 37,
							},
						},
						Text: "database",
					},
				},
				prebody: []byte(`apiVersion: apiextensions.crossplane.io/v1
kind: CompositeResourceDefinition
metadata:
  name: compositepostgresqlinstances..example.org`),
			},
			want: want{
				content: []byte(`apiVersion: apiextensions.crossplane.io/v1
kind: CompositeResourceDefinition
metadata:
  name: compositepostgresqlinstances.database.example.org`),
			},
		},
		"InvalidRangeProvided": {
			args: args{
				uri: span.URIFromPath("/ws/xrd.yaml"),
				changes: []protocol.TextDocumentContentChangeEvent{
					{
						Text: "d",
					},
				},
				prebody: []byte(`apiVersion: apiextensions.crossplane.io/v1
kind: CompositeResourceDefinition
metadata:
  name: compositepostgresqlinstances.atabase.example.org`),
			},
			want: want{
				err: errors.New(errInvalidRange),
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			ws, _ := NewWorkspace("/ws", "/cache")

			ws.snapshot.ws[tc.args.uri.Filename()] = tc.args.prebody

			body, err := ws.updateChanges(ctx, tc.args.uri, tc.args.changes)

			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nUpdateChanges(...): -want error, +got error:\n%s", tc.reason, diff)
			}

			if diff := cmp.Diff(tc.want.content, body); diff != "" {
				t.Errorf("\n%s\nUpdateChanges(...): -want error, +got error:\n%s", tc.reason, diff)
			}
		})
	}
}
