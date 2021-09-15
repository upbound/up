package license

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"testing"

	"github.com/crossplane/crossplane-runtime/pkg/test"
	"github.com/google/go-cmp/cmp"
	"github.com/pkg/errors"
	"github.com/upbound/dmv/api"
	"github.com/upbound/up/internal/http/mocks"
)

func TestGetAccessKey(t *testing.T) {
	errBoom := errors.New("boom")
	defaultURL, _ := url.Parse("https://test.com")
	bearerToken := "bearerToken"
	successToken := "TOKEN"
	successSig := "SIG"

	ctx := context.Background()

	type want struct {
		response Response
	}

	cases := map[string]struct {
		reason    string
		client    api.HttpRequestDoer
		want      want
		clientErr error
		err       error
	}{
		"SuccessfulAuth": {
			reason: "A successful call to DMV should return the expected access key",

			client: &mocks.MockClient{
				DoFn: func(req *http.Request) (*http.Response, error) {
					return &http.Response{
						StatusCode: http.StatusOK,
						Header: map[string][]string{
							"Content-Type": {"application/json"},
						},
						Body: ioutil.NopCloser(
							bytes.NewReader([]byte(
								fmt.Sprintf(
									`{"key": "%s", "signature": "%s"}`,
									successToken,
									successSig,
								),
							),
							),
						),
					}, nil
				}},
			want: want{
				response: Response{
					AccessKey: successToken,
					Signature: successSig,
				},
			},
		},

		"ErrAuthFailed": {
			reason: "If call to dmv fails an error should be returned.",
			client: &mocks.MockClient{
				DoFn: func(req *http.Request) (*http.Response, error) {
					return nil, errBoom
				}},
			err: errors.Wrap(errBoom, errGetAccessKey),
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {

			client, err := api.NewClientWithResponses(defaultURL.String(), api.WithHTTPClient(tc.client))
			if diff := cmp.Diff(tc.clientErr, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nRun(...): -want error, +got error:\n%s", tc.reason, diff)
			}

			p := dmv{
				client: client,
			}

			resp, err := p.GetAccessKey(ctx, bearerToken, "version")
			if diff := cmp.Diff(tc.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nRun(...): -want error, +got error:\n%s", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.response, resp); diff != "" {
				t.Errorf("\n%s\nconstructAuth(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}
