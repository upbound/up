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

package license

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
	path = "/v1/accessKey/%s/%s:%s"

	errGetAccessKey = "failed to acquire key"
)

// Response is the response returned from a successful access key request
type Response struct {
	AccessKey string `json:"key"`
	Signature string `json:"signature"`
}

// Provider defines a license provider
type Provider interface {
	GetAccessKey(context.Context, string, string) (Response, error)
}

// NewProvider constructs a new dmv provider
func NewProvider(modifiers ...ProviderModifierFn) Provider {

	p := &dmv{
		client: &http.Client{},
	}

	for _, m := range modifiers {
		m(p)
	}

	return p
}

type dmv struct {
	client   uphttp.Client
	endpoint *url.URL

	orgID     string
	productID string
}

// ProviderModifierFn modifies the provider.
type ProviderModifierFn func(*dmv)

// WithEndpoint sets endpoint for the license provider.
func WithEndpoint(endpoint *url.URL) ProviderModifierFn {
	return func(u *dmv) {
		u.endpoint = endpoint
	}
}

// WithOrgID sets orgID for the license provider.
func WithOrgID(orgID string) ProviderModifierFn {
	return func(u *dmv) {
		u.orgID = orgID
	}
}

// WithProductID sets productID for the license provider.
func WithProductID(productID string) ProviderModifierFn {
	return func(u *dmv) {
		u.productID = productID
	}
}

func (d *dmv) GetAccessKey(ctx context.Context, token, version string) (Response, error) {

	d.endpoint.Path = fmt.Sprintf(path, d.orgID, d.productID, version)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, d.endpoint.String(), nil)
	if err != nil {
		return Response{}, errors.Wrap(err, errGetAccessKey)
	}

	req.Header.Set("Content-Type", "application/json")
	// add authorization header to the req
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", token))

	res, err := d.client.Do(req)
	if err != nil {
		return Response{}, errors.Wrap(err, errGetAccessKey)
	}
	defer res.Body.Close() // nolint:errcheck

	b, err := io.ReadAll(res.Body)
	if err != nil {
		return Response{}, errors.Wrap(err, errGetAccessKey)
	}

	var resp Response
	if err := json.Unmarshal(b, &resp); err != nil {
		return Response{}, errors.Wrap(err, errGetAccessKey)
	}

	return resp, err
}
