package image

import (
	"context"

	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
)

// MockFetcher is an image fetcher that returns its configured values.
type MockFetcher struct {
	tags []string
	img  v1.Image
	dsc  *v1.Descriptor
	err  error
}

// NewMockFetcher constructs a new mock fetcher.
func NewMockFetcher(opts ...MockOption) *MockFetcher {
	f := &MockFetcher{}
	for _, o := range opts {
		o(f)
	}
	return f
}

// MockOption modifies the mock fetcher.
type MockOption func(*MockFetcher)

// WithTags sets the tags for the mock fetcher.
func WithTags(tags []string) MockOption {
	return func(m *MockFetcher) {
		m.tags = tags
	}
}

// WithError sets the error for the mock fetcher.
func WithError(err error) MockOption {
	return func(m *MockFetcher) {
		m.err = err
	}
}

// WithImage sets the image for the mock fetcher.
func WithImage(img v1.Image) MockOption {
	return func(m *MockFetcher) {
		m.img = img
	}
}

// WithDescriptor sets the descriptor for the mock fetcher.
func WithDescriptor(dsc *v1.Descriptor) MockOption {
	return func(m *MockFetcher) {
		m.dsc = dsc
	}
}

// Fetch returns the configured error.
func (m *MockFetcher) Fetch(ctx context.Context, ref name.Reference, secrets ...string) (v1.Image, error) {
	return m.img, m.err
}

// Head returns the configured error.
func (m *MockFetcher) Head(ctx context.Context, ref name.Reference, secrets ...string) (*v1.Descriptor, error) {
	return m.dsc, m.err
}

// Tags returns the configured tags or if none exist then error.
func (m *MockFetcher) Tags(ctx context.Context, ref name.Reference, secrets ...string) ([]string, error) {
	return m.tags, m.err
}
