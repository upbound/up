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
	"net/http"
	"net/url"
	"strings"

	"github.com/deepmap/oapi-codegen/pkg/securityprovider"
	"github.com/pkg/errors"

	"github.com/upbound/dmv/api"
)

const (
	errGetAccessKey = "failed to acquire key"
)

// Response is the response returned from a successful access key request
type Response struct {
	AccessKey string `json:"access_key"`
	Signature string `json:"signature"`
}

// Provider defines a license provider
type Provider interface {
	GetAccessKey(context.Context, string, string) (Response, error)
}

// NewProvider constructs a new dmv provider
func NewProvider(modifiers ...ProviderModifierFn) Provider {

	p := &dmv{}

	for _, m := range modifiers {
		m(p)
	}

	client, _ := api.NewClientWithResponses(
		p.endpoint.String(),
	)

	p.client = client

	return p
}

type dmv struct {
	client   api.ClientWithResponsesInterface
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

	// err always returns nil
	tokenProvider, _ := securityprovider.NewSecurityProviderBearerToken(token)

	res, err := d.client.GetAccessKeyWithResponse(
		ctx,
		d.orgID,
		d.productID,
		convertVersion(version),
		tokenProvider.Intercept,
	)

	if err != nil {
		return Response{}, errors.Wrap(err, errGetAccessKey)
	}

	if res.StatusCode() != http.StatusOK {
		return Response{}, errors.Wrap(err, errGetAccessKey)
	}

	return Response{
		AccessKey: res.JSON200.Key,
		Signature: res.JSON200.Signature,
	}, nil
}

func convertVersion(version string) string {
	return strings.ReplaceAll(version, ".", "-")
}
