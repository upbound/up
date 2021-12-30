package snapshot

import (
	"bufio"
	"bytes"
	"context"
	"io"
	"path/filepath"

	"k8s.io/apiextensions-apiserver/pkg/apiserver/validation"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	ext "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions"
	extv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apimachyaml "k8s.io/apimachinery/pkg/util/yaml"
	k8syaml "sigs.k8s.io/yaml"

	"github.com/crossplane/crossplane-runtime/pkg/logging"
	"github.com/crossplane/crossplane/apis/pkg/v1beta1"

	"github.com/upbound/up/internal/xpkg"
	"github.com/upbound/up/internal/xpkg/dep/manager"
	mxpkg "github.com/upbound/up/internal/xpkg/dep/marshaler/xpkg"
	"github.com/upbound/up/internal/xpkg/snapshot/validator"
	"github.com/upbound/up/internal/xpkg/snapshot/validator/meta"
	"github.com/upbound/up/internal/xpkg/workspace"
)

type DepManager interface {
	View(context.Context, []v1beta1.Dependency) (*manager.View, error)
	Versions(context.Context, v1beta1.Dependency) ([]string, error)
}

// Snapshot provides a unified point in time snapshot of the details needed to
// perform operations on an xpkg project. These details include:
// - currently parsed workspace files
// - external dependencies per the crossplane.yaml in the xpkg project
type Snapshot struct {
	m   DepManager
	w   *workspace.Workspace
	log logging.Logger

	wsview *workspace.View
	// packages includes the parsed packages from the defined package
	// dependencies.
	packages map[string]*mxpkg.ParsedPackage
	// validators includes validators for both the workspace as well as
	// the external dependencies defined in the crossplane.yaml.
	validators map[schema.GroupVersionKind]validator.Validator
}

// New constructs a new Snapshot at the given workdir, with the supplied logger.
func New(workdir string, l logging.Logger, opts ...Option) (*Snapshot, error) {
	s := &Snapshot{
		// log is not set to a default so that we can supply the consistently
		// share it with the supporting subsystems.
		log: l,
	}

	m, err := manager.New(manager.WithLogger(l))
	if err != nil {
		return nil, err
	}

	s.m = m

	w, err := workspace.New(workdir, workspace.WithLogger(l))
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

// init initializes the snapshot with needed details from the workspace
// and dep manager.
func (s *Snapshot) init() error {
	if err := s.w.Parse(); err != nil {
		return err
	}

	s.wsview = s.w.View()

	// initialize snapshot validators with workspace validators
	if err := s.loadWSValidators(); err != nil {
		return err
	}

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
		for k, v := range extView.Validators() {
			s.validators[k] = v
		}

		s.packages = extView.Packages()
	}

	return nil
}

// Option modifies a snapshot.
type Option func(*Snapshot)

// WithDepManager overrides the default dependency manager with the provided
// manager.
func WithDepManager(m DepManager) Option {
	return func(s *Snapshot) {
		s.m = m
	}
}

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

// loadWSValidators processes the details from the parsed workspace, extracting
// the corresponding validators and applying them to the workspace.
func (s *Snapshot) loadWSValidators() error { // nolint:gocyclo
	validators := map[schema.GroupVersionKind]validator.Validator{}

	for f, d := range s.wsview.FileDetails() {
		yr := apimachyaml.NewYAMLReader(bufio.NewReader(bytes.NewReader(d.Body)))
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
			if isMeta(f.SpanURI().Filename()) {
				// just need to get gvk from this file
				mobj := &unstructured.Unstructured{}
				if err := k8syaml.Unmarshal(b, mobj); err != nil {
					continue
				}
				m, err := meta.New(s.m, s.packages)
				if err != nil {
					continue
				}

				validators[mobj.GetObjectKind().GroupVersionKind()] = m
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
	}

	s.validators = validators
	return nil
}

// isMeta tests whether the supplied filename matches our expected meta filename.
func isMeta(filename string) bool {
	return filepath.Base(filename) == xpkg.MetaFile
}
