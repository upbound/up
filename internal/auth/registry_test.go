package auth

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

	"github.com/upbound/up/internal/http/mocks"
)

func TestGetToken(t *testing.T) {
	errBoom := errors.New("boom")
	defaultURL, _ := url.Parse("https://test.com")
	successToken := "SUCCESS"

	ctx := context.Background()

	type want struct {
		response Response
	}

	cases := map[string]struct {
		reason   string
		provider *upboundRegistry
		want     want
		err      error
	}{
		"SuccessfulAuth": {
			reason: "Providing a valid id and password should return a valid auth request.",
			provider: &upboundRegistry{
				client: &mocks.MockClient{
					DoFn: func(req *http.Request) (*http.Response, error) {
						return &http.Response{
							StatusCode: http.StatusOK,
							Body:       ioutil.NopCloser(bytes.NewReader([]byte(fmt.Sprintf(`{"access_token": "%s"}`, successToken)))),
						}, nil
					},
				},
				username: "cool-user",
				password: "cool-pass",
				endpoint: defaultURL,
			},
			want: want{
				response: Response{
					AccessToken: successToken,
				},
			},
		},
		"ErrAuthFailed": {
			reason: "If call to registry fails an error should be returned.",
			provider: &upboundRegistry{
				client: &mocks.MockClient{
					DoFn: func(req *http.Request) (*http.Response, error) {
						return nil, errBoom
					},
				},
				username: "cool-user",
				password: "cool-pass",
				endpoint: defaultURL,
			},
			err: errors.Wrap(errBoom, errAuthFailed),
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			resp, err := tc.provider.GetToken(ctx)
			if diff := cmp.Diff(tc.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nRun(...): -want error, +got error:\n%s", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.response, resp); diff != "" {
				t.Errorf("\n%s\nconstructAuth(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}
