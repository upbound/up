/*
Copyright 2020 The Crossplane Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package composite

import (
	"context"
	"time"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"

	"github.com/crossplane/crossplane-runtime/pkg/event"
	"github.com/crossplane/crossplane-runtime/pkg/logging"
	"github.com/crossplane/crossplane-runtime/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
	"github.com/crossplane/crossplane-runtime/pkg/resource/unstructured/composite"

	v1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
	env "github.com/crossplane/crossplane/internal/controller/apiextensions/composite/environment"
)

const (
	shortWait = 30 * time.Second
	longWait  = 1 * time.Minute
	timeout   = 2 * time.Minute
)

// Error strings
const (
	errGet          = "cannot get composite resource"
	errUpdate       = "cannot update composite resource"
	errUpdateStatus = "cannot update composite resource status"
	errSelectComp   = "cannot select Composition"
	errFetchComp    = "cannot fetch Composition"
	errConfigure    = "cannot configure composite resource"
	errPublish      = "cannot publish connection details"
	errRenderCD     = "cannot render composed resource"
	errValidate     = "refusing to use invalid Composition"
	errAssociate    = "cannot associate composed resources with Composition resource templates"

	errFmtRender = "cannot render composed resource from resource template at index %d"

	errFmtPatchEnvironment = "cannot apply environment patch at index %d"
)

// Event reasons.
const (
	reasonResolve event.Reason = "SelectComposition"
	reasonCompose event.Reason = "ComposeResources"
	reasonPublish event.Reason = "PublishConnectionSecret"
)

// ControllerName returns the recommended name for controllers that use this
// package to reconcile a particular kind of composite resource.
func ControllerName(name string) string {
	return "composite/" + name
}

// ConnectionSecretFilterer returns a set of allowed keys.
type ConnectionSecretFilterer interface {
	GetConnectionSecretKeys() []string
}

// A ConnectionPublisher publishes the supplied ConnectionDetails for the
// supplied resource. Publishers must handle the case in which the supplied
// ConnectionDetails are empty.
type ConnectionPublisher interface {
	// PublishConnection details for the supplied resource. Publishing must be
	// additive; i.e. if details (a, b, c) are published, subsequently
	// publishing details (b, c, d) should update (b, c) but not remove a.
	// Returns 'published' if the publish was not a no-op.
	PublishConnection(ctx context.Context, o resource.ConnectionSecretOwner, c managed.ConnectionDetails) (published bool, err error)
}

// A ConnectionPublisherFn publishes the supplied ConnectionDetails for the
// supplied resource.
type ConnectionPublisherFn func(ctx context.Context, o resource.ConnectionSecretOwner, c managed.ConnectionDetails) (published bool, err error)

// PublishConnection details for the supplied resource.
func (fn ConnectionPublisherFn) PublishConnection(ctx context.Context, o resource.ConnectionSecretOwner, c managed.ConnectionDetails) (published bool, err error) {
	return fn(ctx, o, c)
}

// A CompositionSelector selects a composition reference.
type CompositionSelector interface {
	SelectComposition(ctx context.Context, cr resource.Composite) error
}

// A CompositionSelectorFn selects a composition reference.
type CompositionSelectorFn func(ctx context.Context, cr resource.Composite) error

// SelectComposition for the supplied composite resource.
func (fn CompositionSelectorFn) SelectComposition(ctx context.Context, cr resource.Composite) error {
	return fn(ctx, cr)
}

// A CompositionFetcher fetches an appropriate Composition for the supplied
// composite resource.
type CompositionFetcher interface {
	Fetch(ctx context.Context, cr resource.Composite) (*v1.Composition, error)
}

// A CompositionFetcherFn fetches an appropriate Composition for the supplied
// composite resource.
type CompositionFetcherFn func(ctx context.Context, cr resource.Composite) (*v1.Composition, error)

// Fetch an appropriate Composition for the supplied Composite resource.
func (fn CompositionFetcherFn) Fetch(ctx context.Context, cr resource.Composite) (*v1.Composition, error) {
	return fn(ctx, cr)
}

// A Configurator configures a composite resource using its composition.
type Configurator interface {
	Configure(ctx context.Context, cr resource.Composite, cp *v1.Composition) error
}

// A ConfiguratorFn configures a composite resource using its composition.
type ConfiguratorFn func(ctx context.Context, cr resource.Composite, cp *v1.Composition) error

// Configure the supplied composite resource using its composition.
func (fn ConfiguratorFn) Configure(ctx context.Context, cr resource.Composite, cp *v1.Composition) error {
	return fn(ctx, cr, cp)
}

// A Renderer is used to render a composed resource.
type Renderer interface {
	Render(ctx context.Context, cp resource.Composite, cd resource.Composed, t v1.ComposedTemplate, env *env.Environment) error
}

// A RendererFn may be used to render a composed resource.
type RendererFn func(ctx context.Context, cp resource.Composite, cd resource.Composed, t v1.ComposedTemplate, env *env.Environment) error

// Render the supplied composed resource using the supplied composite resource
// and template as inputs.
func (fn RendererFn) Render(ctx context.Context, cp resource.Composite, cd resource.Composed, t v1.ComposedTemplate, env *env.Environment) error {
	return fn(ctx, cp, cd, t, env)
}

// A ReadinessChecker checks whether a composed resource is ready or not.
type ReadinessChecker interface {
	IsReady(ctx context.Context, cd resource.Composed, t v1.ComposedTemplate) (ready bool, err error)
}

// A ReadinessCheckerFn checks whether a composed resource is ready or not.
type ReadinessCheckerFn func(ctx context.Context, cd resource.Composed, t v1.ComposedTemplate) (ready bool, err error)

// IsReady reports whether a composed resource is ready or not.
func (fn ReadinessCheckerFn) IsReady(ctx context.Context, cd resource.Composed, t v1.ComposedTemplate) (ready bool, err error) {
	return fn(ctx, cd, t)
}

// A CompositionRequest is a request to compose resources.
// It should be treated as immutable.
type CompositionRequest struct {
	Composition *v1.Composition
	Environment *env.Environment
}

// A CompositionResult is the result of the composition process.
type CompositionResult struct {
	Composed          []ComposedResource
	ConnectionDetails managed.ConnectionDetails
	Events            []event.Event
}

// A Composer composes (i.e. creates, updates, or deletes) resources given the
// supplied composite resource and composition request.
type Composer interface {
	Compose(ctx context.Context, xr resource.Composite, req CompositionRequest) ([]ComposedResourceState, error)
}

// A ComposerFn composes resources.
type ComposerFn func(ctx context.Context, xr resource.Composite, req CompositionRequest) ([]ComposedResourceState, error)

// Compose resources.
func (fn ComposerFn) Compose(ctx context.Context, xr resource.Composite, req CompositionRequest) ([]ComposedResourceState, error) {
	return fn(ctx, xr, req)
}

// ReconcilerOption is used to configure the Reconciler.
type ReconcilerOption func(*Reconciler)

// WithLogger specifies how the Reconciler should log messages.
func WithLogger(log logging.Logger) ReconcilerOption {
	return func(r *Reconciler) {
		r.log = log
	}
}

// WithCompositionValidator specifies how the Reconciler should validate
// Compositions.
func WithCompositionValidator(v CompositionValidator) ReconcilerOption {
	return func(r *Reconciler) {
		r.composition.CompositionValidator = v
	}
}

// WithCompositionTemplateAssociator specifies how the Reconciler should
// associate composition templates with composed resources.
func WithCompositionTemplateAssociator(a CompositionTemplateAssociator) ReconcilerOption {
	return func(r *Reconciler) {
		r.composition.CompositionTemplateAssociator = a
	}
}

// WithComposer specifies how the Reconciler should compose resources.
func WithComposer(c Composer) ReconcilerOption {
	return func(r *Reconciler) {
		r.resource = c
	}
}

// WithConfigurator specifies how the Reconciler should configure
// composite resources using their composition.
func WithConfigurator(c Configurator) ReconcilerOption {
	return func(r *Reconciler) {
		r.composite.Configurator = c
	}
}

type composition struct {
	CompositionValidator
	CompositionTemplateAssociator
}

type compositeResource struct {
	Configurator
}

// NewReconciler returns a new Reconciler of composite resources.
func NewReconciler(of resource.CompositeKind, opts ...ReconcilerOption) *Reconciler {
	nc := func() resource.Composite {
		return composite.New(composite.WithGroupVersionKind(schema.GroupVersionKind(of)))
	}

	r := &Reconciler{
		newComposite: nc,

		composition: composition{
			CompositionValidator: ValidationChain{
				CompositionValidatorFn(RejectMixedTemplates),
				CompositionValidatorFn(RejectDuplicateNames),
			},
			CompositionTemplateAssociator: NewGarbageCollectingAssociator(),
		},

		composite: compositeResource{
			Configurator: NewConfiguratorChain(NewAPINamingConfigurator(), NewAPIConfigurator()),
		},

		resource: NewPTComposer(),

		log: logging.NewNopLogger(),
	}

	for _, f := range opts {
		f(r)
	}
	return r
}

// A Reconciler reconciles composite resources.
type Reconciler struct {
	newComposite func() resource.Composite

	composition composition
	composite   compositeResource

	resource Composer

	log logging.Logger
}

// composedRenderState is a wrapper around a composed resource that tracks whether
// it was successfully rendered or not, together with a list of patches defined
// on its template that have been applied (not filtered out).
type composedRenderState struct {
	resource       resource.Composed
	rendered       bool
	appliedPatches []v1.Patch
}

// Reconcile a composite resource.
func (r *Reconciler) Reconcile(ctx context.Context, comp *v1.Composition) ([]ComposedResourceState, error) { //nolint:gocyclo
	// NOTE(negz): Like most Reconcile methods, this one is over our cyclomatic
	// complexity goal. Be wary when adding branches, and look for functionality
	// that could be reasonably moved into an injected dependency.

	cr := r.newComposite()
	cr.SetName(PlaceholderName)
	cr.SetUID(types.UID(PlaceholderUID))

	// TODO(negz): Composition validation should be handled by a validation
	// webhook, not by this controller.
	if err := r.composition.Validate(comp); err != nil {
		r.log.Debug(errValidate, "error", err)
		return nil, err
	}

	if err := r.composite.Configure(ctx, cr, comp); err != nil {
		r.log.Debug(errConfigure, "error", err)
		return nil, err
	}

	// Inline PatchSets from Composition Spec before composing resources

	cds, err := r.resource.Compose(ctx, cr, CompositionRequest{Composition: comp, Environment: nil})
	if err != nil {
		r.log.Debug(errRenderCD, "error", err)
	}

	return cds, nil
}

// filterToXRPatches selects patches defined in composed templates,
// whose type is one of the XR-targeting patches
// (e.g. v1.PatchTypeToCompositeFieldPath or v1.PatchTypeCombineToComposite)
func filterToXRPatches(tas []TemplateAssociation) []v1.Patch {
	filtered := make([]v1.Patch, 0, len(tas))
	for _, ta := range tas {
		filtered = append(filtered, filterPatches(ta.Template.Patches,
			patchTypesToXR()...)...)
	}
	return filtered
}
