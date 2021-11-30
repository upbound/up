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
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"io/fs"
	"path/filepath"
	"strings"
	"sync"

	"github.com/goccy/go-yaml"
	"github.com/goccy/go-yaml/ast"
	"github.com/goccy/go-yaml/parser"
	"github.com/goccy/go-yaml/token"
	"github.com/golang/tools/lsp/protocol"
	"github.com/golang/tools/span"
	"github.com/sourcegraph/go-lsp"
	"github.com/spf13/afero"

	ext "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions"
	extv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apiextensions-apiserver/pkg/apiserver/validation"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	apimachyaml "k8s.io/apimachinery/pkg/util/yaml"
	verrors "k8s.io/kube-openapi/pkg/validation/errors"
	"k8s.io/kube-openapi/pkg/validation/validate"
	k8syaml "sigs.k8s.io/yaml"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	xpextv1 "github.com/crossplane/crossplane/apis/apiextensions/v1"

	"github.com/upbound/up/internal/xpkg"
	"github.com/upbound/up/internal/xpkg/dep/cache"
	"github.com/upbound/up/internal/xpkg/dep/manager"
	"github.com/upbound/up/internal/xpkg/dep/workspace"
	"github.com/upbound/up/internal/xpls/validator"
)

// paths to extract GVK and name from objects that conform to Kubernetes
// standard.
var (
	compResources *yaml.Path
	compBase      *yaml.Path
)

const (
	errCompositionResources = "resources in Composition are malformed"
	errFileBodyNotFound     = "could not find corresponding file body for %s"
	errNoChangesSupplied    = "no content changes provided"
	errInvalidFileURI       = "invalid path supplied"
	errInvalidRange         = "invalid range supplied"
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

// A PackageNode represents a concrete node in an xpkg.
// TODO(hasheddan): PackageNode should be refactored into separate
// implementations for each node type (e.g. XRD, Composition, CRD, etc.).
type PackageNode struct {
	ast        ast.Node
	fileName   string
	obj        *unstructured.Unstructured
	dependants map[NodeIdentifier]struct{}
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
	return p.obj.GroupVersionKind()
}

// GetObject returns the GroupVersionKind of this node.
func (p *PackageNode) GetObject() metav1.Object {
	return p.obj
}

// NodeIdentifier is the unique identifier of a node in a workspace.
type NodeIdentifier struct {
	name string
	gvk  schema.GroupVersionKind
}

// nodeID constructs a NodeIdentifier from name and GVK.
func nodeID(name string, gvk schema.GroupVersionKind) NodeIdentifier {
	return NodeIdentifier{
		name: name,
		gvk:  gvk,
	}
}

// A Node is a single object in the package workspace graph.
type Node interface {
	GetAST() ast.Node
	GetFileName() string
	GetDependants() []NodeIdentifier
	GetGVK() schema.GroupVersionKind
	GetObject() metav1.Object
}

// A Validator validates data and returns a validation result.
type Validator interface {
	Validate(data interface{}) *validate.Result
}

// A Workspace represents a single xpkg workspace. It is safe for multi-threaded
// use.
type Workspace struct {
	fs afero.Fs

	// The absolute path of the workspace.
	root string

	mu sync.RWMutex

	m *manager.Manager

	// The node cache maintains a set of nodes present in a workspace. A node
	// identifier is a combination of its GVK and name.
	nodes map[NodeIdentifier]Node

	snapshot *Snapshot

	// The validator cache maintains a set of validators loaded from the package cache.
	validators map[schema.GroupVersionKind]Validator
}

// Snapshot represents the workspace for the given snapshot.
// TODO(@tnthornton) we probably want to version the shapshots in order
// to handle multiple inbound changes.
type Snapshot struct {
	// map of full filename to file contents
	ws map[string][]byte
}

// A WorkspaceOpt modifies the configuration of a workspace.
type WorkspaceOpt func(*Workspace)

// WithFS sets the filesystem for the workspace.
func WithFS(fs afero.Fs) WorkspaceOpt {
	return func(w *Workspace) {
		w.fs = fs
	}
}

// NewWorkspace constructs a new Workspace by loading validators from the
// package cache. A workspace must be parsed before it can be validated.
func NewWorkspace(root, cacheRoot string, opts ...WorkspaceOpt) (*Workspace, error) {
	w := &Workspace{
		fs:         afero.NewOsFs(),
		root:       root,
		nodes:      map[NodeIdentifier]Node{},
		snapshot:   &Snapshot{ws: map[string][]byte{}},
		validators: map[schema.GroupVersionKind]Validator{},
	}

	c, err := cache.NewLocal(cache.WithRoot(cacheRoot))
	if err != nil {
		return nil, err
	}

	m, err := manager.New(manager.WithCache(c))
	if err != nil {
		return nil, err
	}

	w.m = m

	for _, o := range opts {
		o(w)
	}
	return w, nil
}

// IsMeta tests whether the supplied filename matches our expected meta filename.
func (w *Workspace) IsMeta(filename string) bool {
	return filepath.Base(filename) == xpkg.MetaFile
}

// Parse parses all objects in a workspace and stores them in the node cache.
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
		// We attempt to parse subsequent documents if we encounter an error
		// in a preceding one.
		// TODO(hasheddan): errors should be aggregated and returned as
		// diagnostics.

		b, err := afero.ReadFile(w.fs, p)
		if err != nil {
			return err
		}

		// add file contents to our inmem workspace
		w.snapshot.ws[p] = b

		_ = w.parseFile(p)
		return nil
	})
}

// ParseFile acquires a write lock then calls parseFile.
func (w *Workspace) ParseFile(path string) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.parseFile(path)
}

// parseFile parses all YAML objects at the given path and updates the workspace
// node cache.
func (w *Workspace) parseFile(path string) error {

	f, err := parser.ParseBytes(w.snapshot.ws[path], parser.ParseComments)
	if err != nil {
		return err
	}
	for _, doc := range f.Docs {
		if _, err := w.parseDoc(doc, path); err != nil {
			// We attempt to parse subsequent documents if we encounter an error
			// in a preceding one.
			// TODO(hasheddan): errors should be aggregated and returned as
			// diagnostics.
			continue
		}
	}
	return nil
}

// parseDoc recursively parses a YAML document into PackageNodes. Embedded nodes
// are added to the parent's list of dependants.
func (w *Workspace) parseDoc(n ast.Node, path string) (NodeIdentifier, error) {
	b, err := n.MarshalYAML()
	if err != nil {
		return NodeIdentifier{}, err
	}
	obj := &unstructured.Unstructured{}
	// NOTE(hasheddan): unmarshal returns an error if Kind is not defined.
	// TODO(hasheddan): we cannot make use of strict unmarshal to identify
	// extraneous fields because we don't have the underlying Go types. In
	// the future, we would like to provide warnings on fields that are
	// extraneous, but we will likely need to augment the OpenAPI validation
	// to do so.
	if err := k8syaml.Unmarshal(b, obj); err != nil {
		return NodeIdentifier{}, err
	}
	dependants := map[NodeIdentifier]struct{}{}
	// NOTE(hasheddan): if we are at document root (i.e. this is a
	// DocumentNode), we must set the underlying ast.Node to the document body
	// so that we can access child nodes generically in validation.
	if doc, ok := n.(*ast.DocumentNode); ok {
		n = doc.Body
	}
	if obj.GetKind() == xpextv1.CompositionKind {
		resNode, err := compResources.FilterNode(n)
		if err != nil {
			return NodeIdentifier{}, err
		}
		seq, ok := resNode.(*ast.SequenceNode)
		if !ok {
			// NOTE(hasheddan): if the Composition's resources field is not a
			// sequence node, we skip parsing embedded resources because the
			// Composition itself is malformed.
			return NodeIdentifier{}, errors.New(errCompositionResources)
		}
		for _, s := range seq.Values {
			sNode, err := compBase.FilterNode(s)
			if err != nil {
				// TODO(hasheddan): surface this error as a diagnostic.
				continue
			}
			id, err := w.parseDoc(sNode, path)
			if err != nil {
				// TODO(hasheddan): surface this error as a diagnostic.
				continue
			}
			dependants[id] = struct{}{}
		}
	}
	// TODO(hasheddan): if this is an embedded resource we don't have a name so
	// we should form a deterministic name based on its parent Composition.
	id := nodeID(obj.GetName(), obj.GroupVersionKind())
	w.nodes[id] = &PackageNode{
		ast:        n,
		fileName:   path,
		obj:        obj,
		dependants: dependants,
	}
	return id, nil
}

// A NodeFilterFn filters the node on which we perform validation.
type NodeFilterFn func(nodes map[NodeIdentifier]Node) []Node

// AllNodes does not filter out any nodes in the workspace.
func AllNodes(nodes map[NodeIdentifier]Node) []Node {
	ns := make([]Node, len(nodes))
	i := 0
	for _, n := range nodes {
		ns[i] = n
		i++
	}
	return ns
}

// Validate performs validation on all filtered nodes and returns diagnostics
// for any validation errors encountered.
// TODO(hasheddan): consider decoupling forming diagnostics from getting
// validation errors.
func (w *Workspace) Validate(fn NodeFilterFn) ([]lsp.Diagnostic, error) { // nolint:gocyclo
	w.mu.RLock()
	defer w.mu.RUnlock()
	diags := []lsp.Diagnostic{}
	for _, n := range fn(w.nodes) {
		gvk := n.GetGVK()
		v, ok := w.validators[gvk]
		if !ok {
			continue
			// TODO(@tnthornton) if we can't find the validator for the given node, we should
			// surface that error in the editor.
		}
		diags = append(diags, validationDiagnostics(v.Validate(n.GetObject()), n.GetAST(), n.GetGVK())...)
	}
	return diags, nil
}

// validationDiagnostics generates language server diagnostics from validation
// errors.
// TODO(@tnthornton) this function is getting pretty complex. We should work
// towards breaking it up.
func validationDiagnostics(res *validate.Result, n ast.Node, gvk schema.GroupVersionKind) []lsp.Diagnostic { // nolint:gocyclo
	diags := []lsp.Diagnostic{}
	for _, err := range res.Errors {
		var e *verror
		switch et := err.(type) {
		case *verrors.Validation:
			e = &verror{
				code:    et.Code(),
				message: fmt.Sprintf("%s (%s)", et.Error(), gvk),
				name:    et.Name,
			}
		case *validator.MetaValidaton:
			e = &verror{
				code:    et.Code(),
				message: et.Error(),
				name:    et.Name,
			}
		default:
			// found an error type we weren't expecting
			continue
		}

		// TODO(hasheddan): handle the case where error occurs and we
		// don't have a valid path.
		if len(e.name) > 0 && e.name != "." {
			errPath := e.name
			if e.code == verrors.RequiredFailCode {
				errPath = e.name[:strings.LastIndex(e.name, ".")]
			}
			// TODO(hasheddan): a general error should be surfaced if we
			// cannot determine the location in the document causing the
			// error.
			path, err := yaml.PathString("$." + errPath)
			if err != nil {
				continue
			}
			node, err := path.FilterNode(n)
			if err != nil {
				continue
			}
			tok := node.GetToken()
			if tok != nil {
				startCh, endCh := tok.Position.Column-1, 0

				// end character can be unmatched if we have doublequotes
				switch tok.Type { // nolint:exhaustive
				case token.DoubleQuoteType:
					endCh = tok.Position.Column + len(tok.Value) + 1
				default:
					endCh = tok.Position.Column + len(tok.Value) - 1
				}
				// TODO(hasheddan): token position reflects file line
				// and column by NOT being zero-indexed, but VSCode
				// interprets ranges with zero-indexing. We should
				// develop a more robust solution for this conversion.
				diags = append(diags, lsp.Diagnostic{
					Range: lsp.Range{
						Start: lsp.Position{
							Line:      tok.Position.Line - 1,
							Character: startCh,
						},
						End: lsp.Position{
							Line:      tok.Position.Line - 1,
							Character: endCh,
						},
					},
					Message:  e.Error(),
					Severity: lsp.Error,
					Source:   serverName,
				})
			}
		}
	}
	return diags
}

// LoadCacheValidators loads the validators corresponding to the external
// dependencies defined in the dep cache (if there is a meta file in the
// project root).
func (w *Workspace) LoadCacheValidators() error {
	depWS, err := workspace.New(
		workspace.WithWorkingDir(w.root),
	)
	if err != nil {
		return err
	}

	// grab external dependency validators only if a meta file is defined.
	if depWS.MetaExists() {
		deps, err := depWS.DependsOn()
		if err != nil {
			return err
		}

		snap, err := w.m.Snapshot(context.Background(), deps)
		if err != nil {
			return err
		}

		// add external deps to the set of validators for the workspace.
		for k, v := range snap.View() {
			w.validators[k] = v
		}
	}
	return nil
}

// LoadValidators loads all validators from the specified location.
// TODO(hasheddan): we currently assume that the cache holds objects in their
// CRD form, but it is more likely that we will need to extract them from
// packages.
// TODO(hasheddan): consider refactoring this to allow for sourcing validators
// from a generic YAML reader, similar to the package parser.
func (w *Workspace) LoadValidators(path string) error { // nolint:gocyclo
	validators := map[schema.GroupVersionKind]Validator{}

	if err := afero.Walk(w.fs, path, func(p string, info fs.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		// NOTE(hasheddan): path is cleaned before being passed to our walk
		// function.
		f, err := w.fs.Open(p) // nolint:gosec
		if err != nil {
			return err
		}
		defer f.Close() // nolint:errcheck,gosec
		yr := apimachyaml.NewYAMLReader(bufio.NewReader(f))
		for {
			b, err := yr.Read()
			if err != nil && err != io.EOF {
				return err
			}
			if err == io.EOF {
				break
			}
			if len(b) == 0 {
				continue
			}
			if w.IsMeta(p) {
				// just need to get gvk from this file
				meta := &unstructured.Unstructured{}
				if err := k8syaml.Unmarshal(b, meta); err != nil {
					continue
				}
				v, err := validator.NewMeta(w.m)
				if err != nil {
					continue
				}

				validators[meta.GetObjectKind().GroupVersionKind()] = v
				continue
			}
			// TODO(hasheddan): handle v1beta1 CRDs, as well as all XRD API versions.
			crd := &extv1.CustomResourceDefinition{}
			if err := k8syaml.Unmarshal(b, crd); err != nil {
				// Skip YAML document if we cannot unmarshal to v1 CRD.
				continue
			}
			internal := &ext.CustomResourceDefinition{}
			if err := extv1.Convert_v1_CustomResourceDefinition_To_apiextensions_CustomResourceDefinition(crd, internal, nil); err != nil {
				return err
			}
			// NOTE(hasheddan): If top-level validation is set, we use it for
			// all versions and continue.
			if internal.Spec.Validation != nil {
				sv, _, err := validation.NewSchemaValidator(internal.Spec.Validation)
				if err != nil {
					return err
				}
				for _, v := range internal.Spec.Versions {
					validators[schema.GroupVersionKind{
						Group:   internal.Spec.Group,
						Version: v.Name,
						Kind:    internal.Spec.Names.Kind,
					}] = sv
				}
				continue
			}
			for _, v := range internal.Spec.Versions {
				sv, _, err := validation.NewSchemaValidator(v.Schema)
				if err != nil {
					return err
				}
				validators[schema.GroupVersionKind{
					Group:   internal.Spec.Group,
					Version: v.Name,
					Kind:    internal.Spec.Names.Kind,
				}] = sv
			}
		}
		return nil
	}); err != nil {
		return err
	}
	// NOTE(hasheddan): we wait to acquire the lock until we have finished
	// constructing all validators so that we can continue performing validation
	// while loading any new validators.
	w.mu.Lock()
	defer w.mu.Unlock()
	for k, v := range validators {
		w.validators[k] = v
	}
	return nil
}

// updateContent updates the current immem content representation for the provided file uri.
func (w *Workspace) updateContent(ctx context.Context, uri span.URI, changes []protocol.TextDocumentContentChangeEvent) error {
	if len(changes) == 0 {
		return errors.New(errNoChangesSupplied)
	}

	body, err := w.updateChanges(ctx, uri, changes)
	if err != nil {
		return err
	}

	w.mu.Lock()
	defer w.mu.Unlock()
	w.snapshot.ws[uri.Filename()] = body

	return nil
}

// updateChanges updates the body mapped to the given filename uri based on the
// provided changes. This is strongly inspired by how gopls injects spans into
// content bodies.
func (w *Workspace) updateChanges(_ context.Context, uri span.URI, changes []protocol.TextDocumentContentChangeEvent) ([]byte, error) {
	if uri == "" {
		return nil, errors.New(errInvalidFileURI)
	}

	content, ok := w.snapshot.ws[uri.Filename()]
	if !ok {
		return nil, fmt.Errorf(errFileBodyNotFound, uri.Filename())
	}

	for _, c := range changes {
		converter := span.NewContentConverter(uri.Filename(), content)
		m := &protocol.ColumnMapper{
			URI:       uri,
			Converter: converter,
			Content:   content,
		}

		if c.Range == nil {
			return nil, errors.New(errInvalidRange)
		}

		spn, err := m.RangeSpan(*c.Range)
		if err != nil {
			return nil, err
		}

		if !spn.HasOffset() {
			return nil, errors.New(errInvalidRange)
		}

		start, end := spn.Start().Offset(), spn.End().Offset()
		if end < start {
			return nil, errors.New(errInvalidRange)
		}

		// inject changes into surrounding content body
		var buf bytes.Buffer
		buf.Write(content[:start])
		buf.WriteString(c.Text)
		buf.Write(content[end:])
		content = buf.Bytes()

	}

	return content, nil
}

// verror normalizes the different validation error types that we work with.
type verror struct {
	code    int32
	message string
	name    string
}

func (e *verror) Error() string {
	return e.message
}
