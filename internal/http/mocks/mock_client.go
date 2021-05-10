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
