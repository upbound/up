package snapshot

import (
	"context"

	"github.com/crossplane/crossplane-runtime/pkg/logging"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/upbound/up/internal/xpkg/dep/manager"
	"github.com/upbound/up/internal/xpkg/dep/marshaler/xpkg"
	"github.com/upbound/up/internal/xpkg/workspace"
	"github.com/upbound/up/internal/xpls/validator"
)

// Snapshot provides a unified point in time snapshot of the details needed to
// perform operations on an xpkg project. These details include:
// - currently parsed workspace files
// - external dependencies per the crossplane.yaml in the xpkg project
type Snapshot struct {
	m   *manager.Manager
	w   *workspace.Workspace
	log logging.Logger

	// packages includes the parsed packages from the defined package
	// dependencies.
	packages map[string]*xpkg.ParsedPackage
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

	localView := s.w.View()

	deps, err := localView.Meta().DependsOn()
	if err != nil {
		return err
	}
	extView, err := s.m.View(context.Background(), deps)
	if err != nil {
		return err
	}

	// initialize snapshot validators with workspace validators
	s.validators = localView.Validators()
	// add external dependency validators to snapshot validators
	for k, v := range extView.Validators() {
		s.validators[k] = v
	}

	s.packages = extView.Packages()

	return nil
}

// Option modifies a snapshot.
type Option func(*Snapshot)

// WithDepManager overrides the default dependency manager with the provided
// manager.
func WithDepManager(m *manager.Manager) Option {
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
func (s *Snapshot) Package(name string) *xpkg.ParsedPackage {
	return s.packages[name]
}
