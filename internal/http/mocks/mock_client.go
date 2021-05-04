package mocks

import (
	"net/http"

	uphttp "github.com/upbound/up/internal/http"
)

var _ uphttp.Client = &MockClient{}

// MockClient is a mock HTTP client.
type MockClient struct {
	DoFn func(req *http.Request) (*http.Response, error)
}

// Do calls the underlying DoFn.
func (m *MockClient) Do(req *http.Request) (*http.Response, error) {
	return m.DoFn(req)
}
