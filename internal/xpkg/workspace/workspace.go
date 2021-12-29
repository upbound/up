package workspace

import (
	"bufio"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sync"

	"github.com/goccy/go-yaml"
	"github.com/goccy/go-yaml/ast"
	"github.com/goccy/go-yaml/parser"
	"github.com/golang/tools/lsp/protocol"
	"github.com/spf13/afero"

	ext "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions"
	extv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apiextensions-apiserver/pkg/apiserver/validation"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	apimachyaml "k8s.io/apimachinery/pkg/util/yaml"
	k8syaml "sigs.k8s.io/yaml"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	xparser "github.com/crossplane/crossplane-runtime/pkg/parser"
	xpextv1 "github.com/crossplane/crossplane/apis/apiextensions/v1"

	"github.com/upbound/up/internal/xpkg"
	pyaml "github.com/upbound/up/internal/xpkg/parser/yaml"
	"github.com/upbound/up/internal/xpkg/workspace/meta"
	"github.com/upbound/up/internal/xpls/validator"
)

// paths to extract GVK and name from objects that conform to Kubernetes
// standard.
var (
	compResources *yaml.Path
	compBase      *yaml.Path
)

const (
	fileProtocol    = "file://"
	fileProtocolFmt = "file://%s"
	yamlExt         = ".yaml"

	errCompositionResources = "resources in Composition are malformed"
	errFileBodyNotFound     = "could not find corresponding file body for %s"
	errInvalidFileURI       = "invalid path supplied"
	errInvalidRange         = "invalid range supplied"
	errMetaFileDoesNotExist = "meta file does not exist"
	errNoChangesSupplied    = "no content changes provided"
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

// Workspace --
type Workspace struct {
	// fs represents the filesystem the workspace resides in.
	fs afero.Fs
	// metaLocation denotes the place the meta file exists at the time the
	// workspace was created.
	metaLocation string

	mu sync.RWMutex
	// parser is the parser used for parsing packages.
	parser *xparser.PackageParser
	// root represents the "root" of the workspace filesystem.
	root string
	view *View
}

// New creates a new Workspace instance.
func New(root string, opts ...Option) (*Workspace, error) {
	w := &Workspace{
		fs:   afero.NewOsFs(),
		root: root,
		// Default metaLocation to the root. If a pre-existing meta file exists,
		// metaLocation will be updating accordingly during workspace parse.
		metaLocation: root,
		view: &View{
			meta:         &meta.Meta{},
			nodes:        make(map[NodeIdentifier]Node),
			uriToDetails: make(map[protocol.DocumentURI]*Details),
			validators:   make(map[schema.GroupVersionKind]validator.Validator),
		},
	}

	p, err := pyaml.New()
	if err != nil {
		return nil, err
	}

	w.parser = p

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

// Meta returns the metafile located within the workspace.
func (w *Workspace) Meta() *meta.Meta {
	return w.view.meta
}

// Write writes the supplied Meta details to the fs.
func (w *Workspace) Write(m *meta.Meta) error {
	b, err := m.Bytes()
	if err != nil {
		return err
	}

	return afero.WriteFile(w.fs, filepath.Join(w.metaLocation, xpkg.MetaFile), b, os.ModePerm)
}

// Parse parses the full workspace in order to hydrate the workspace's View.
func (w *Workspace) Parse() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if err := afero.Walk(w.fs, w.root, func(p string, info fs.FileInfo, err error) error {
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
		w.view.uriToDetails[protocol.URIFromPath(p)] = &Details{
			Body:    b,
			NodeIDs: make(map[NodeIdentifier]struct{}),
		}

		_ = w.parseFile(p)
		return nil
	}); err != nil {
		return err
	}

	// TODO(@tnthornton) loading the validators this way means we end up
	// iterating over the workspace twice. Fix that.
	return w.loadValidators(w.root)
}

// parseFile parses all YAML objects at the given path and updates the workspace
// node cache.
func (w *Workspace) parseFile(path string) error {
	details, ok := w.view.uriToDetails[protocol.URIFromPath(path)]
	if !ok {
		return errors.New(errInvalidFileURI)
	}

	f, err := parser.ParseBytes(details.Body, parser.ParseComments)
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
func (w *Workspace) parseDoc(n ast.Node, path string) (NodeIdentifier, error) { //nolint:gocyclo
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

	// if this is node for the meta file, note it in the workspace for easy lookups.
	if isMeta(path) {
		w.metaLocation = path
		w.view.meta = meta.New(obj)
	}

	w.view.nodes[id] = &PackageNode{
		ast:        n,
		fileName:   path,
		obj:        obj,
		dependants: dependants,
	}

	w.appendID(path, id)

	return id, nil
}

func (w *Workspace) loadValidators(path string) error { // nolint:gocyclo
	validators := map[schema.GroupVersionKind]validator.Validator{}

	if err := afero.Walk(w.fs, path, func(p string, info fs.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		if filepath.Ext(p) != yamlExt {
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
			if isMeta(p) {
				// just need to get gvk from this file
				mobj := &unstructured.Unstructured{}
				if err := k8syaml.Unmarshal(b, mobj); err != nil {
					continue
				}
				// TODO(@tnthornton) make these validators not rely on
				// dep manager directly.
				// m, err := meta.New(w.m, w.snapshot.packages)
				// if err != nil {
				// 	continue
				// }

				// validators[mobj.GetObjectKind().GroupVersionKind()] = m
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

	w.view.validators = validators
	return nil
}

func (w *Workspace) appendID(path string, id NodeIdentifier) {
	uri := protocol.URIFromPath(path)
	curr, ok := w.view.uriToDetails[uri]
	if !ok {
		w.view.uriToDetails[uri] = &Details{
			NodeIDs: map[NodeIdentifier]struct{}{
				id: {},
			},
		}
		return
	}
	curr.NodeIDs[id] = struct{}{}

	w.view.uriToDetails[uri] = curr
}

// isMeta tests whether the supplied filename matches our expected meta filename.
func isMeta(filename string) bool {
	return filepath.Base(filename) == xpkg.MetaFile
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
	meta         *meta.Meta
	uriToDetails map[protocol.DocumentURI]*Details
	nodes        map[NodeIdentifier]Node
	validators   map[schema.GroupVersionKind]validator.Validator
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
	GetObject() metav1.Object
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
