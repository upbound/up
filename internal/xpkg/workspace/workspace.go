// Copyright 2022 Upbound Inc
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

package workspace

import (
	"bytes"
	"context"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/goccy/go-yaml"
	"github.com/goccy/go-yaml/ast"
	"github.com/goccy/go-yaml/parser"
	"github.com/golang/tools/span"
	"github.com/spf13/afero"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	k8syaml "sigs.k8s.io/yaml"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/logging"
	xparser "github.com/crossplane/crossplane-runtime/pkg/parser"
	xpextv1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
	pkgmetav1 "github.com/crossplane/crossplane/apis/pkg/meta/v1"

	"github.com/upbound/up/internal/xpkg"
	pyaml "github.com/upbound/up/internal/xpkg/parser/yaml"
	"github.com/upbound/up/internal/xpkg/workspace/meta"
)

// paths to extract GVK and name from objects that conform to Kubernetes
// standard.
var (
	compResources *yaml.Path
	compBase      *yaml.Path
)

const (
	yamlExt = ".yaml"

	errCompositionResources = "resources in Composition are malformed"
	errInvalidFileURI       = "invalid path supplied"
	errInvalidPackage       = "invalid package; more than one meta file supplied"
)

// builds static YAML path strings ahead of usage.
func init() {
	var err error
	compResources, err = yaml.PathString("$.spec.resources")
	if err != nil {
		panic(err)
	}
	compBase, err = yaml.PathString("$.base")
	if err != nil {
		panic(err)
	}
}

// Workspace provides APIs for interacting with the current project workspace.
type Workspace struct {
	// fs represents the filesystem the workspace resides in.
	fs afero.Fs

	log logging.Logger

	mu sync.RWMutex
	// root represents the "root" of the workspace filesystem.
	root string
	view *View
}

// New creates a new Workspace instance.
func New(root string, opts ...Option) (*Workspace, error) {
	w := &Workspace{
		fs:   afero.NewOsFs(),
		log:  logging.NewNopLogger(),
		root: root,
		view: &View{
			examples: make(map[schema.GroupVersionKind][]Node),
			// Default metaLocation to the root. If a pre-existing meta file exists,
			// metaLocation will be updating accordingly during workspace parse.
			metaLocation: root,
			nodes:        make(map[NodeIdentifier]Node),
			uriToDetails: make(map[span.URI]*Details),
			xrClaimRefs:  make(map[schema.GroupVersionKind]schema.GroupVersionKind),
		},
	}

	p, err := pyaml.New()
	if err != nil {
		return nil, err
	}

	w.view.parser = p

	// apply overrides if they exist
	for _, o := range opts {
		o(w)
	}

	return w, nil
}

// Option represents an option that can be applied to Workspace.
type Option func(*Workspace)

// WithFS overrides the Workspace's filesystem with the supplied filesystem.
func WithFS(fs afero.Fs) Option {
	return func(w *Workspace) {
		w.fs = fs
	}
}

// WithLogger overrides the default logger for the Workspace with the supplied
// logger.
func WithLogger(l logging.Logger) Option {
	return func(w *Workspace) {
		w.log = l
	}
}

// Write writes the supplied Meta details to the fs.
func (w *Workspace) Write(m *meta.Meta) error {
	b, err := m.Bytes()
	if err != nil {
		return err
	}

	return afero.WriteFile(w.fs, filepath.Join(w.view.metaLocation, xpkg.MetaFile), b, os.ModePerm)
}

// Parse parses the full workspace in order to hydrate the workspace's View.
func (w *Workspace) Parse() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	return afero.Walk(w.fs, w.root, func(p string, info fs.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}

		if filepath.Ext(p) != yamlExt {
			return nil
		}
		// We attempt to parse subsequent documents if we encounter an error
		// in a preceding one.
		// TODO(hasheddan): errors should be aggregated and returned as
		// diagnostics.

		b, err := afero.ReadFile(w.fs, p)
		if err != nil {
			return err
		}

		// add file contents to our inmem workspace
		w.view.uriToDetails[span.URIFromPath(p)] = &Details{
			Body:    b,
			NodeIDs: make(map[NodeIdentifier]struct{}),
		}

		_ = w.view.ParseFile(p)
		return nil
	})
}

// View returns the Workspace's View. Note: this will only exist _after_
// the Workspace has been parsed.
func (w *Workspace) View() *View {
	return w.view
}

// ParseFile parses all YAML objects at the given path and updates the workspace
// node cache.
func (v *View) ParseFile(path string) error {
	details, ok := v.uriToDetails[span.URIFromPath(path)]
	if !ok {
		return errors.New(errInvalidFileURI)
	}

	f, err := parser.ParseBytes(details.Body, parser.ParseComments)
	if err != nil {
		return err
	}
	for _, doc := range f.Docs {
		if doc.Body != nil {
			pCtx := parseContext{
				node:     doc,
				path:     path,
				rootNode: true,
			}
			if _, err := v.parseDoc(pCtx); err != nil {
				// We attempt to parse subsequent documents if we encounter an error
				// in a preceding one.
				// TODO(hasheddan): errors should be aggregated and returned as
				// diagnostics.
				continue
			}
		}
	}
	return nil
}

type parseContext struct {
	docBytes []byte
	node     ast.Node
	obj      unstructured.Unstructured
	path     string
	rootNode bool
}

// parseDoc recursively parses a YAML document into PackageNodes. Embedded nodes
// are added to the parent's list of dependants.
func (v *View) parseDoc(pCtx parseContext) (NodeIdentifier, error) { //nolint:gocyclo
	b, err := pCtx.node.MarshalYAML()
	if err != nil {
		return NodeIdentifier{}, err
	}
	pCtx.docBytes = b

	var obj unstructured.Unstructured
	// NOTE(hasheddan): unmarshal returns an error if Kind is not defined.
	// TODO(hasheddan): we cannot make use of strict unmarshal to identify
	// extraneous fields because we don't have the underlying Go types. In
	// the future, we would like to provide warnings on fields that are
	// extraneous, but we will likely need to augment the OpenAPI validation
	// to do so.
	if err := k8syaml.Unmarshal(b, &obj); err != nil {
		return NodeIdentifier{}, err
	}
	pCtx.obj = obj
	// NOTE(hasheddan): if we are at document root (i.e. this is a
	// DocumentNode), we must set the underlying ast.Node to the document body
	// so that we can access child nodes generically in validation.
	if doc, ok := pCtx.node.(*ast.DocumentNode); ok {
		pCtx.node = doc.Body
	}

	switch obj.GetKind() {
	case xpextv1.CompositeResourceDefinitionKind:
		if err := v.parseXRD(pCtx); err != nil {
			return NodeIdentifier{}, err
		}
	case xpextv1.CompositionKind:
		if err := v.parseComposition(pCtx); err != nil {
			return NodeIdentifier{}, err
		}
	case pkgmetav1.ConfigurationKind:
		if err := v.parseMeta(pCtx); err != nil {
			return NodeIdentifier{}, err
		}
	case pkgmetav1.ProviderKind:
		if err := v.parseMeta(pCtx); err != nil {
			return NodeIdentifier{}, err
		}
	default:
		v.parseExample(pCtx)
	}
	// TODO(hasheddan): if this is an embedded resource we don't have a name so
	// we should form a deterministic name based on its parent Composition.
	id := nodeID(obj.GetName(), obj.GroupVersionKind())

	v.nodes[id] = &PackageNode{
		ast:      pCtx.node,
		fileName: pCtx.path,
		gvk:      obj.GroupVersionKind(),
		obj:      &obj,
	}

	if pCtx.rootNode {
		v.appendID(pCtx.path, id)
	}

	return id, nil
}

func (v *View) parseComposition(ctx parseContext) error {
	var cp xpextv1.Composition
	if err := k8syaml.Unmarshal(ctx.docBytes, &cp); err != nil {
		// we have a composition but failed to unmarshal it, skip for now.
		return nil // nolint:nilerr
	}

	resNode, err := compResources.FilterNode(ctx.node)
	if err != nil {
		return err
	}
	seq, ok := resNode.(*ast.SequenceNode)
	if !ok {
		// NOTE(hasheddan): if the Composition's resources field is not a
		// sequence node, we skip parsing embedded resources because the
		// Composition itself is malformed.
		return errors.New(errCompositionResources)
	}

	dependants := map[NodeIdentifier]struct{}{}

	for _, s := range seq.Values {
		// process ComposedTemplate
		b, err := s.MarshalYAML()
		if err != nil {
			return err
		}

		var ct xpextv1.ComposedTemplate
		if err := k8syaml.Unmarshal(b, &ct); err != nil {
			return err
		}

		// recurse into resource[i].base
		sNode, err := compBase.FilterNode(s)
		if err != nil {
			// TODO(hasheddan): surface this error as a diagnostic.
			continue
		}
		ctx.node = sNode
		ctx.rootNode = false

		id, err := v.parseDoc(ctx)
		if err != nil {
			// TODO(hasheddan): surface this error as a diagnostic.
			continue
		}
		dependants[id] = struct{}{}
	}
	return nil
}

func (v *View) parseExample(ctx parseContext) {
	// NOTE(@tnthornton): we handle example claims specially so that we have
	// them available for CompositeTemplate validation.
	if strings.Contains(filepath.Dir(ctx.path), "example") {
		curr, ok := v.examples[ctx.obj.GroupVersionKind()]
		if !ok {
			curr = make([]Node, 0)
		}

		curr = append(curr, &PackageNode{
			ast:      ctx.node,
			fileName: ctx.path,
			gvk:      ctx.obj.GroupVersionKind(),
			obj:      &ctx.obj,
		})

		v.examples[ctx.obj.GroupVersionKind()] = curr
	}
}

func (v *View) parseMeta(ctx parseContext) error {
	v.metaLocation = filepath.Dir(ctx.path)
	p, err := v.parser.Parse(context.Background(), io.NopCloser(bytes.NewReader(ctx.docBytes)))
	if err != nil {
		return err
	}

	if len(p.GetMeta()) != 1 {
		return errors.New(errInvalidPackage)
	}

	v.meta = meta.New(p.GetMeta()[0])
	return nil
}

func (v *View) parseXRD(ctx parseContext) error {
	var xrd xpextv1.CompositeResourceDefinition
	if err := k8syaml.Unmarshal(ctx.docBytes, &xrd); err != nil {
		return err
	}

	v.xrClaimRefs[xrd.GetCompositeGroupVersionKind()] = xrd.GetClaimGroupVersionKind()
	return nil
}

func (v *View) appendID(path string, id NodeIdentifier) {
	uri := span.URIFromPath(path)
	curr, ok := v.uriToDetails[uri]
	if !ok {
		v.uriToDetails[uri] = &Details{
			NodeIDs: map[NodeIdentifier]struct{}{
				id: {},
			},
		}
		return
	}
	curr.NodeIDs[id] = struct{}{}

	v.uriToDetails[uri] = curr
}

// nodeID constructs a NodeIdentifier from name and GVK.
func nodeID(name string, gvk schema.GroupVersionKind) NodeIdentifier {
	return NodeIdentifier{
		name: name,
		gvk:  gvk,
	}
}

// View represents the current processed view of the workspace.
type View struct {
	// examples holds a quick access map of GVK -> []Nodes representing the
	// example/**/*.yaml claims for the package.
	examples map[schema.GroupVersionKind][]Node
	// parser is the parser used for parsing packages.
	parser *xparser.PackageParser
	// metaLocation denotes the place the meta file exists at the time the
	// workspace was created.
	metaLocation string
	meta         *meta.Meta
	uriToDetails map[span.URI]*Details
	nodes        map[NodeIdentifier]Node
	// xrClaimRefs defines an look up from XR GVK -> Claim GVK (if one is defined).
	xrClaimRefs map[schema.GroupVersionKind]schema.GroupVersionKind
}

// FileDetails returns the map of file details found within the parsed workspace.
func (v *View) FileDetails() map[span.URI]*Details {
	return v.uriToDetails
}

// Meta returns the View's Meta.
func (v *View) Meta() *meta.Meta {
	return v.meta
}

// MetaLocation returns the meta file's location (on disk) in the current View.
func (v *View) MetaLocation() string {
	return v.metaLocation
}

// Nodes returns the View's Nodes.
func (v *View) Nodes() map[NodeIdentifier]Node {
	return v.nodes
}

// Examples returns the View's Nodes corresponding to the files found under
// /examples.
func (v *View) Examples() map[schema.GroupVersionKind][]Node {
	return v.examples
}

// XRClaimsRefs returns a map of XR GVK -> XRC GVK.
func (v *View) XRClaimsRefs() map[schema.GroupVersionKind]schema.GroupVersionKind {
	return v.xrClaimRefs
}

// Details represent file specific details.
type Details struct {
	Body    []byte
	NodeIDs map[NodeIdentifier]struct{}
}

// A Node is a single object in the package workspace graph.
type Node interface {
	GetAST() ast.Node
	GetFileName() string
	GetDependants() []NodeIdentifier
	GetGVK() schema.GroupVersionKind
	GetObject() runtime.Object
}

// NodeIdentifier is the unique identifier of a node in a workspace.
type NodeIdentifier struct {
	name string
	gvk  schema.GroupVersionKind
}

// A PackageNode represents a concrete node in an xpkg.
// TODO(hasheddan): PackageNode should be refactored into separate
// implementations for each node type (e.g. XRD, Composition, CRD, etc.).
type PackageNode struct {
	ast      ast.Node
	fileName string
	gvk      schema.GroupVersionKind
	obj      runtime.Object
}

// GetAST gets the YAML AST node for this package node.
func (p *PackageNode) GetAST() ast.Node {
	return p.ast
}

// GetFileName gets the name of the file for this node.
func (p *PackageNode) GetFileName() string {
	return p.fileName
}

// GetDependants gets the set of nodes dependant on this node.
// TODO(hasheddan): this method signature may change depending on how we want to
// construct the node graph for a workspace.
func (p *PackageNode) GetDependants() []NodeIdentifier {
	return nil
}

// GetGVK returns the GroupVersionKind of this node.
func (p *PackageNode) GetGVK() schema.GroupVersionKind {
	return p.gvk
}

// GetObject returns the runtime.Object for this node.
func (p *PackageNode) GetObject() runtime.Object {
	return p.obj
}
