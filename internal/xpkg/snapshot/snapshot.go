package snapshot

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"sync"

	"github.com/goccy/go-yaml/ast"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer/json"
	"k8s.io/kube-openapi/pkg/validation/validate"

	apimachyaml "k8s.io/apimachinery/pkg/util/yaml"
	verrors "k8s.io/kube-openapi/pkg/validation/errors"
	k8syaml "sigs.k8s.io/yaml"

	"github.com/crossplane/crossplane-runtime/pkg/logging"
	"github.com/crossplane/crossplane/apis/pkg/v1beta1"
	"github.com/goccy/go-yaml"
	"github.com/goccy/go-yaml/token"
	"github.com/golang/tools/lsp/protocol"
	"github.com/golang/tools/span"

	"github.com/upbound/up/internal/xpkg"
	"github.com/upbound/up/internal/xpkg/dep/cache"
	"github.com/upbound/up/internal/xpkg/dep/manager"
	mxpkg "github.com/upbound/up/internal/xpkg/dep/marshaler/xpkg"
	"github.com/upbound/up/internal/xpkg/validator"
	"github.com/upbound/up/internal/xpkg/validator/meta"
	"github.com/upbound/up/internal/xpkg/validator/object"
	"github.com/upbound/up/internal/xpkg/workspace"
)

const (
	serverName = "xpls"

	errFileBodyNotFound  = "could not find corresponding file body for %s"
	errInvalidFileURI    = "invalid path supplied"
	errInvalidNodeID     = "invalid node id supplied"
	errInvalidRange      = "invalid range supplied"
	errNoChangesSupplied = "no content changes provided"
)

// DepManager defines the API necessary for working with the dependency manager.
type DepManager interface {
	View(context.Context, []v1beta1.Dependency) (*manager.View, error)
	Versions(context.Context, v1beta1.Dependency) ([]string, error)
	Watch() <-chan cache.Event
}

// Snapshot provides a unified point in time snapshot of the details needed to
// perform operations on an xpkg project. These details include:
// - currently parsed workspace files
// - external dependencies per the crossplane.yaml in the xpkg project
type Snapshot struct {
	// TODO synchonize access to the snapshot using channels so that we don't
	// need this lock.
	mu sync.RWMutex

	m   DepManager
	w   *workspace.Workspace
	log logging.Logger

	objScheme *runtime.Scheme
	// packages includes the parsed packages from the defined package
	// dependencies.
	packages map[string]*mxpkg.ParsedPackage
	// validators includes validators for both the workspace as well as
	// the external dependencies defined in the crossplane.yaml.
	validators map[schema.GroupVersionKind]validator.Validator
	wsview     *workspace.View
}

// Factory is used to "stamp out" Snapshots while allowing
// a shared set of references.
type Factory struct {
	log logging.Logger
	m   DepManager

	workdir string
	// initialize the object scheme once for the factory as this won't change
	// during its lifecycle.
	objScheme *runtime.Scheme
}

// NewFactory returns a new Snapshot Factory instance.
func NewFactory(workdir string, opts ...FactoryOption) (*Factory, error) {
	f := &Factory{
		log:     logging.NewNopLogger(),
		workdir: workdir,
	}

	m, err := manager.New(manager.WithLogger(f.log))
	if err != nil {
		return nil, err
	}

	f.m = m

	objScheme, err := xpkg.BuildObjectScheme()
	if err != nil {
		return nil, err
	}

	f.objScheme = objScheme

	for _, o := range opts {
		o(f)
	}

	return f, nil
}

// New constructs a new Snapshot at the given workdir, with the supplied logger.
func (f *Factory) New(opts ...Option) (*Snapshot, error) {
	s := &Snapshot{
		// log is not set to a default so that we can supply the consistently
		// share it with the supporting subsystems.
		log:        f.log,
		objScheme:  f.objScheme,
		validators: make(map[schema.GroupVersionKind]validator.Validator),
	}

	// use the manager instance from the Factory
	s.m = f.m

	w, err := workspace.New(f.workdir, workspace.WithLogger(s.log))
	if err != nil {
		return nil, err
	}

	s.w = w

	for _, o := range opts {
		o(s)
	}

	if err := s.init(); err != nil {
		return nil, err
	}

	return s, nil
}

// WatchExt provides a channel for callers to subscribe to external changes
// that affect the Factory. For example, cache changes.
func (f *Factory) WatchExt() <-chan cache.Event {
	return f.m.Watch()
}

// init initializes the snapshot with needed details from the workspace
// and dep manager.
func (s *Snapshot) init() error {
	if err := s.w.Parse(); err != nil {
		return err
	}

	s.wsview = s.w.View()

	meta := s.wsview.Meta()
	if meta != nil {
		deps, err := s.wsview.Meta().DependsOn()
		if err != nil {
			return err
		}
		extView, err := s.m.View(context.Background(), deps)
		if err != nil {
			return err
		}

		// add external dependency validators to snapshot validators
		for gvk, v := range extView.Validators() {
			s.validators[gvk] = v
		}

		s.packages = extView.Packages()
	}

	// initialize snapshot validators with workspace validators
	if err := s.loadWSValidators(); err != nil {
		return err
	}

	return nil
}

// FactoryOption modifies a Factory.
type FactoryOption func(*Factory)

// WithDepManager overrides the default dependency manager with the provided
// manager.
func WithDepManager(m DepManager) FactoryOption {
	return func(f *Factory) {
		f.m = m
	}
}

// WithLogger overrides the default logger with the provided Logger.
func WithLogger(l logging.Logger) FactoryOption {
	return func(f *Factory) {
		f.log = l
	}
}

// Option modifies a Snapshot.
type Option func(*Snapshot)

// WithWorkspace overrides the default workspace with the provided workspace.
func WithWorkspace(w *workspace.Workspace) Option {
	return func(s *Snapshot) {
		s.w = w
	}
}

// Validator returns the Validator corresponding to the provided GVK within
// the Snapshot, if one exists. Nil otherwise.
func (s *Snapshot) Validator(gvk schema.GroupVersionKind) validator.Validator {
	return s.validators[gvk]
}

// Package returns the ParsedPackage corresponding to the supplied package name
// as defined in the crossplane.yaml, if one exists. Nil otherwise.
func (s *Snapshot) Package(name string) *mxpkg.ParsedPackage {
	return s.packages[name]
}

// ReParseFile re-parses the file at the given path. This is only useful in
// cases where our snapshot representation has changed prior to the given file
// being saved.
func (s *Snapshot) ReParseFile(path string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.wsview.ParseFile(path)
}

// UpdateContent updates the current immem content representation for the provided file uri.
func (s *Snapshot) UpdateContent(ctx context.Context, uri span.URI, changes []protocol.TextDocumentContentChangeEvent) error {
	if len(changes) == 0 {
		return errors.New(errNoChangesSupplied)
	}

	body, err := s.updateChanges(ctx, uri, changes)
	if err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	details := s.wsview.FileDetails()[uri]
	details.Body = body

	return nil
}

// updateChanges updates the body mapped to the given filename uri based on the
// provided changes. This is strongly inspired by how gopls injects spans into
// content bodies.
func (s *Snapshot) updateChanges(_ context.Context, uri span.URI, changes []protocol.TextDocumentContentChangeEvent) ([]byte, error) {
	if uri == "" {
		return nil, errors.New(errInvalidFileURI)
	}

	details, ok := s.wsview.FileDetails()[uri]
	if !ok {
		return nil, fmt.Errorf(errFileBodyNotFound, uri.Filename())
	}

	content := details.Body

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

// loadWSValidators processes the details from the parsed workspace, extracting
// the corresponding validators and applying them to the workspace.
func (s *Snapshot) loadWSValidators() error { // nolint:gocyclo
	wsValidators := map[schema.GroupVersionKind]validator.Validator{}

	for f, d := range s.wsview.FileDetails() {
		yr := apimachyaml.NewYAMLReader(bufio.NewReader(bytes.NewReader(d.Body)))
		do := json.NewSerializerWithOptions(json.DefaultMetaFactory, s.objScheme, s.objScheme, json.SerializerOptions{Yaml: true})
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
			if isMeta(f.Filename()) {
				// just need to get gvk from this file
				mobj := &unstructured.Unstructured{}
				if err := k8syaml.Unmarshal(b, mobj); err != nil {
					continue
				}
				m, err := meta.New(s.m, s.packages)
				if err != nil {
					continue
				}

				wsValidators[mobj.GetObjectKind().GroupVersionKind()] = m
				continue
			}

			o, _, err := do.Decode(b, nil, nil)
			if err != nil {
				// skip YAML document if we cannot unmarshal to runtime.Object
				continue
			}

			validators, err := object.ValidatorsForObj(o)
			if err != nil {
				// skip YAML document if we cannot acquire validators for object
				continue
			}

			for gvk, v := range validators {
				wsValidators[gvk] = v
			}

			// TODO(hasheddan): handle v1beta1 CRDs, as well as all XRD API versions.
			// crd := &extv1.CustomResourceDefinition{}
			// if err := k8syaml.Unmarshal(b, crd); err != nil {
			// 	// Skip YAML document if we cannot unmarshal to v1 CRD.
			// 	continue
			// }
			// internal := &ext.CustomResourceDefinition{}
			// if err := extv1.Convert_v1_CustomResourceDefinition_To_apiextensions_CustomResourceDefinition(crd, internal, nil); err != nil {
			// 	return err
			// }
			// // NOTE(hasheddan): If top-level validation is set, we use it for
			// // all versions and continue.
			// if internal.Spec.Validation != nil {
			// 	sv, _, err := validation.NewSchemaValidator(internal.Spec.Validation)
			// 	if err != nil {
			// 		return err
			// 	}
			// 	for _, v := range internal.Spec.Versions {
			// 		validators[schema.GroupVersionKind{
			// 			Group:   internal.Spec.Group,
			// 			Version: v.Name,
			// 			Kind:    internal.Spec.Names.Kind,
			// 		}] = vcrd.NewV1(sv)
			// 	}
			// 	continue
			// }
			// for _, v := range internal.Spec.Versions {
			// 	sv, _, err := validation.NewSchemaValidator(v.Schema)
			// 	if err != nil {
			// 		return err
			// 	}
			// 	validators[schema.GroupVersionKind{
			// 		Group:   internal.Spec.Group,
			// 		Version: v.Name,
			// 		Kind:    internal.Spec.Names.Kind,
			// 	}] = vcrd.NewV1(sv)
			// }
		}
	}

	for gvk, v := range wsValidators {
		s.validators[gvk] = v
	}

	return nil
}

// isMeta tests whether the supplied filename matches our expected meta filename.
func isMeta(filename string) bool {
	return filepath.Base(filename) == xpkg.MetaFile
}

// ValidateAllFiles performs validations on all files in Snapshot.
func (s *Snapshot) ValidateAllFiles() (map[span.URI][]protocol.Diagnostic, error) {
	results := make(map[span.URI][]protocol.Diagnostic)
	for f := range s.wsview.FileDetails() {
		diags, _ := s.Validate(f)
		results[f] = diags
	}
	return results, nil
}

// Validate performs validation on all filtered nodes and returns diagnostics
// for any validation errors encountered.
// TODO(hasheddan): consider decoupling forming diagnostics from getting
// validation errors.
func (s *Snapshot) Validate(uri span.URI) ([]protocol.Diagnostic, error) { // nolint:gocyclo
	s.mu.RLock()
	defer s.mu.RUnlock()
	diags := []protocol.Diagnostic{}
	details, ok := s.wsview.FileDetails()[uri]
	if !ok {
		return nil, errors.New(errInvalidFileURI)
	}

	for id := range details.NodeIDs {
		n, ok := s.wsview.Nodes()[id]
		if !ok {
			return nil, errors.New(errInvalidNodeID)
		}
		gvk := n.GetGVK()
		v, ok := s.validators[gvk]
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
func validationDiagnostics(res *validate.Result, n ast.Node, gvk schema.GroupVersionKind) []protocol.Diagnostic { // nolint:gocyclo
	diags := []protocol.Diagnostic{}
	for _, err := range res.Errors {
		var e *verror
		switch et := err.(type) {
		case *verrors.Validation:
			e = &verror{
				code:    et.Code(),
				message: fmt.Sprintf("%s (%s)", et.Error(), gvk),
				name:    et.Name,
			}
		case *validator.MetaValidation:
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
			if node == nil {
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
				diags = append(diags, protocol.Diagnostic{
					Range: protocol.Range{
						Start: protocol.Position{
							Line:      uint32(tok.Position.Line - 1),
							Character: uint32(startCh),
						},
						End: protocol.Position{
							Line:      uint32(tok.Position.Line - 1),
							Character: uint32(endCh),
						},
					},
					Message:  e.Error(),
					Severity: protocol.SeverityError,
					Source:   serverName,
				})
			}
		}
	}
	return diags
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
