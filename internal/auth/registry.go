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

package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/pkg/errors"

	uphttp "github.com/upbound/up/internal/http"
)

const (
	path = "/service/token"
	// query params
	serviceKey   = "service"
	serviceValue = "registry-token-service"
	scopeKey     = "scope"
	scopeValue   = "repository:%s/%s:pull"

	errAuthFailed = "unable to acquire access token"
)

// Response is the response that we get from a successful authentication request
type Response struct {
	AccessToken string `json:"access_token"`
	Scope       string `json:"scope"`
	ExpiresIn   int    `json:"expires_in"`
	IssuedAt    string `json:"issued_at"`
}

// Provider defines an auth provider
type Provider interface {
	GetToken(context.Context) (Response, error)
}

// NewProvider constructs a new upboundRegistry provider
func NewProvider(modifiers ...ProviderModifierFn) Provider {

	p := &upboundRegistry{
		client: &http.Client{},
	}

	for _, m := range modifiers {
		m(p)
	}

	return p
}

type upboundRegistry struct {
	client   uphttp.Client
	endpoint *url.URL

	// Auth
	username string
	password string

	// scope
	orgID     string
	productID string
}

// ProviderModifierFn modifies the provider.
type ProviderModifierFn func(*upboundRegistry)

// WithBasicAuth sets the username and password for the auth provider.
func WithBasicAuth(username, password string) ProviderModifierFn {
	return func(u *upboundRegistry) {
		u.username = username
		u.password = password
	}
}

// WithEndpoint sets endpoint for the auth provider.
func WithEndpoint(endpoint *url.URL) ProviderModifierFn {
	return func(u *upboundRegistry) {
		u.endpoint = endpoint
	}
}

// WithOrgID sets orgID for the auth provider.
func WithOrgID(orgID string) ProviderModifierFn {
	return func(u *upboundRegistry) {
		u.orgID = orgID
	}
}

// WithProductID sets productID for the auth provider.
func WithProductID(productID string) ProviderModifierFn {
	return func(u *upboundRegistry) {
		u.productID = productID
	}
}

func (u *upboundRegistry) GetToken(ctx context.Context) (Response, error) {
	u.endpoint.Path = path

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.endpoint.String(), nil)
	if err != nil {
		return Response{}, errors.Wrap(err, errAuthFailed)
	}

	q := req.URL.Query()
	q.Add(serviceKey, serviceValue)
	q.Add(scopeKey, fmt.Sprintf(scopeValue, u.orgID, u.productID))

	req.URL.RawQuery = q.Encode()

	req.Header.Set("Content-Type", "application/json")
	req.SetBasicAuth(u.username, u.password)

	res, err := u.client.Do(req)
	if err != nil {
		return Response{}, errors.Wrap(err, errAuthFailed)
	}
	defer res.Body.Close() // nolint:errcheck

	b, err := io.ReadAll(res.Body)
	if err != nil {
		return Response{}, errors.Wrap(err, errAuthFailed)
	}

	var resp Response
	if err := json.Unmarshal(b, &resp); err != nil {
		return Response{}, errors.Wrap(err, errAuthFailed)
	}

	return resp, err
}
