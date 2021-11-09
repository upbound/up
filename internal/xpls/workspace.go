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
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/goccy/go-yaml"
	"github.com/goccy/go-yaml/ast"
	"github.com/goccy/go-yaml/parser"
	"github.com/sourcegraph/go-lsp"
	ext "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions"
	extv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apiextensions-apiserver/pkg/apiserver/validation"
	"k8s.io/apimachinery/pkg/runtime/schema"
	apimachyaml "k8s.io/apimachinery/pkg/util/yaml"
	verrors "k8s.io/kube-openapi/pkg/validation/errors"
	"k8s.io/kube-openapi/pkg/validation/validate"
	k8syaml "sigs.k8s.io/yaml"
)

// paths to extract GVK and name from objects that conform to Kubernetes
// standard.
var (
	gvPath   *yaml.Path
	kindPath *yaml.Path
	namePath *yaml.Path
)

// builds static YAML path strings ahead of usage.
func init() {
	var err error
	gvPath, err = yaml.PathString("$.apiVersion")
	if err != nil {
		panic(err)
	}
	kindPath, err = yaml.PathString("$.kind")
	if err != nil {
		panic(err)
	}
	namePath, err = yaml.PathString("$.metadata.name")
	if err != nil {
		panic(err)
	}
}

const (
	errMissingValidatorFmt = "could not find validator for node: %s"
	errParseNode           = "failed to parse node"
)

// A PackageNode represents a concrete node in an xpkg.
// TODO(hasheddan): PackageNode should be refactored into separate
// implementations for each node type (e.g. XRD, Composition, CRD, etc.).
type PackageNode struct {
	ast      ast.Node
	nested   []ast.Node
	fileName string
	gvk      schema.GroupVersionKind
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

// GetNested returns the YAML AST nodes for all nested objects in a package
// node.
func (p *PackageNode) GetNested() []ast.Node {
	return p.nested
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
	GetNested() []ast.Node
}

// A Workspace represents a single xpkg workspace. It is safe for multi-threaded
// use.
type Workspace struct {
	// The absolute path of the workspace.
	root string

	mu sync.RWMutex

	// The node cache maintains a set of nodes present in a workspace. A node
	// identifier is a combination of its GVK and name.
	nodes map[NodeIdentifier]Node

	// The validator cache maintains a set of validators loaded from the package cache.
	validators map[schema.GroupVersionKind]*validate.SchemaValidator
}

// NewWorkspace constructs a new Workspace by loading validators from the
// package cache. A workspace must be parsed before it can be validated.
func NewWorkspace(root, cache string) (*Workspace, error) {
	// TODO(hasheddan): currently we load all validators from the schema cache.
	// In the future we will need to selectively and dynamically load validators
	// based on changing dependencies in the crossplane.yaml.
	vlds, err := validatorsFromDir(cache)
	if err != nil {
		return nil, err
	}
	return &Workspace{
		root:       root,
		nodes:      map[NodeIdentifier]Node{},
		validators: vlds,
	}, nil
}

// Parse parses all objects in a workspace and stores them in the node cache.
func (w *Workspace) Parse() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	return fs.WalkDir(os.DirFS(w.root), ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		fileName := filepath.Join(w.root, p)
		return w.parseFile(fileName)
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
	f, err := parser.ParseFile(path, parser.ParseComments)
	if err != nil {
		return err
	}
	for _, doc := range f.Docs {
		// NOTE(hasheddan): currently we just skip a workspace node in the case
		// that we cannot construct its unique identifier.
		gvNode, err := gvPath.FilterNode(doc.Body)
		if err != nil {
			return err
		}
		gvTok := gvNode.GetToken()
		if gvTok == nil {
			continue
		}
		kindNode, err := kindPath.FilterNode(doc.Body)
		if err != nil {
			return err
		}
		kindTok := kindNode.GetToken()
		if kindTok == nil {
			continue
		}
		nameNode, err := namePath.FilterNode(doc.Body)
		if err != nil {
			return err
		}
		nameTok := nameNode.GetToken()
		if nameTok == nil {
			continue
		}
		gvk := schema.FromAPIVersionAndKind(gvTok.Value, kindTok.Value)
		w.nodes[nodeID(nameTok.Value, gvk)] = &PackageNode{
			ast:      doc.Body,
			fileName: path,
			gvk:      gvk,
		}
	}
	return nil
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
		v, ok := w.validators[n.GetGVK()]
		if !ok {
			return nil, errors.Errorf(errMissingValidatorFmt, n.GetGVK())
		}
		var node interface{}
		if err := yaml.NewDecoder(n.GetAST()).Decode(&node); err != nil {
			return nil, errors.Wrap(err, errParseNode)
		}
		for _, err := range v.Validate(node).Errors {
			if err, ok := err.(*verrors.Validation); ok {
				// TODO(hasheddan): handle the case where error occurs and we
				// don't have a valid path.
				vErr := err
				if len(err.Name) > 0 && err.Name != "." {
					errPath := err.Name
					if err.Code() == verrors.RequiredFailCode {
						errPath = err.Name[:strings.LastIndex(err.Name, ".")]
					}
					path, err := yaml.PathString("$." + errPath)
					if err != nil {
						continue
					}
					node, err := path.FilterNode(n.GetAST())
					if err != nil {
						continue
					}
					tok := node.GetToken()
					if tok != nil {
						// TODO(hasheddan): token position reflects file line
						// and column by NOT being zero-indexed, but VSCode
						// interprets ranges with zero-indexing. We should
						// develop a more robust solution for this conversion.
						diags = append(diags, lsp.Diagnostic{
							Range: lsp.Range{
								Start: lsp.Position{
									Line:      tok.Position.Line - 1,
									Character: tok.Position.Column - 1,
								},
								End: lsp.Position{
									Line:      tok.Position.Line - 1,
									Character: tok.Position.Column + len(tok.Value) - 1,
								},
							},
							Message:  vErr.Error(),
							Severity: lsp.Error,
							Source:   serverName,
						})
					}
				}
			}
		}
	}
	return diags, nil
}

// validatorsFromDir loads all validators from the specified directory.
// TODO(hasheddan): we currently assume that the cache holds objects in their
// CRD form, but it is more likely that we will need to extract them from
// packages.
func validatorsFromDir(path string) (map[schema.GroupVersionKind]*validate.SchemaValidator, error) { // nolint:gocyclo
	validators := map[schema.GroupVersionKind]*validate.SchemaValidator{}
	if err := fs.WalkDir(os.DirFS(path), ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		// NOTE(hasheddan): filepath.Join cleans result, so we ignore gosec
		// warning here.
		f, err := os.Open(filepath.Join(path, p)) // nolint:gosec
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
		return nil, err
	}
	return validators, nil
}
