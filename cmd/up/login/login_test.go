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

package login

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/url"
	"testing"
	"testing/iotest"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/test"
	"github.com/golang-jwt/jwt"
	"github.com/google/go-cmp/cmp"
	"github.com/pterm/pterm"

	"github.com/upbound/up/internal/http/mocks"
	"github.com/upbound/up/internal/profile"
	"github.com/upbound/up/internal/upbound"
)

func TestRun(t *testing.T) {
	errBoom := errors.New("boom")
	defaultURL, _ := url.Parse("https://test.com")

	cases := map[string]struct {
		reason string
		cmd    *LoginCmd
		ctx    *upbound.Context
		err    error
	}{
		"ErrorNoUserOrToken": {
			reason: "If neither user or token is provided an error should be returned.",
			cmd:    &LoginCmd{},
			ctx:    &upbound.Context{},
			err:    errors.Wrap(errors.New(errNoUserOrToken), errLoginFailed),
		},
		"ErrLoginFailed": {
			reason: "If Upbound Cloud endpoint is ",
			cmd: &LoginCmd{
				client: &mocks.MockClient{
					DoFn: func(req *http.Request) (*http.Response, error) {
						return nil, errBoom
					},
				},
				Username: "cool-user",
				Password: "cool-pass",
			},
			ctx: &upbound.Context{
				APIEndpoint: defaultURL,
			},
			err: errors.Wrap(errBoom, errLoginFailed),
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			if diff := cmp.Diff(tc.err, tc.cmd.Run(context.TODO(), pterm.DefaultBasicText.WithWriter(io.Discard), tc.ctx), test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nRun(...): -want error, +got error:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestConstructAuth(t *testing.T) {
	type args struct {
		username string
		token    string
		password string
	}
	type want struct {
		pType profile.TokenType
		auth  *auth
	}
	cases := map[string]struct {
		reason string
		args   args
		want   want
		err    error
	}{
		"ErrorNoUserOrToken": {
			reason: "If neither user or token is provided an error should be returned.",
			err:    errors.New(errNoUserOrToken),
		},
		"SuccessfulUser": {
			reason: "Providing a valid id and password should return a valid auth request.",
			args: args{
				username: "cool-user",
				password: "cool-password",
			},
			want: want{
				pType: profile.User,
				auth: &auth{
					ID:       "cool-user",
					Password: "cool-password",
					Remember: true,
				},
			},
		},
		"SuccessfulToken": {
			reason: "Providing a valid id and token should return a valid auth request.",
			args: args{
				token: "eyJ0eXAiOiJKV1QiLCJhbGciOiJIUzI1NiJ9.eyJpc3MiOiJPbmxpbmUgSldUIEJ1aWxkZXIiLCJpYXQiOjE2MTg1MTc5NDMsImV4cCI6MTY1MDA1Mzk0MywiYXVkIjoiaHR0cHM6Ly9kYW5pZWxtYW5ndW0uY29tIiwic3ViIjoiZ2VvcmdlZGFuaWVsbWFuZ3VtQGdtYWlsLmNvbSIsIkpUSSI6Imhhc2hlZGRhbiJ9.zI42wXvwDHiATx9ycECz7JyATTn9P07wN-TRXvtCGcM",
			},
			want: want{
				pType: profile.Token,
				auth: &auth{
					ID:       "hasheddan",
					Password: "eyJ0eXAiOiJKV1QiLCJhbGciOiJIUzI1NiJ9.eyJpc3MiOiJPbmxpbmUgSldUIEJ1aWxkZXIiLCJpYXQiOjE2MTg1MTc5NDMsImV4cCI6MTY1MDA1Mzk0MywiYXVkIjoiaHR0cHM6Ly9kYW5pZWxtYW5ndW0uY29tIiwic3ViIjoiZ2VvcmdlZGFuaWVsbWFuZ3VtQGdtYWlsLmNvbSIsIkpUSSI6Imhhc2hlZGRhbiJ9.zI42wXvwDHiATx9ycECz7JyATTn9P07wN-TRXvtCGcM",
					Remember: true,
				},
			},
		},
		"SuccessfulTokenIgnorePassword": {
			reason: "Providing a valid id and token should return a valid auth request without extraneous password.",
			args: args{
				token:    "eyJ0eXAiOiJKV1QiLCJhbGciOiJIUzI1NiJ9.eyJpc3MiOiJPbmxpbmUgSldUIEJ1aWxkZXIiLCJpYXQiOjE2MTg1MTc5NDMsImV4cCI6MTY1MDA1Mzk0MywiYXVkIjoiaHR0cHM6Ly9kYW5pZWxtYW5ndW0uY29tIiwic3ViIjoiZ2VvcmdlZGFuaWVsbWFuZ3VtQGdtYWlsLmNvbSIsIkpUSSI6Imhhc2hlZGRhbiJ9.zI42wXvwDHiATx9ycECz7JyATTn9P07wN-TRXvtCGcM",
				password: "forget-about-me",
			},
			want: want{
				pType: profile.Token,
				auth: &auth{
					ID:       "hasheddan",
					Password: "eyJ0eXAiOiJKV1QiLCJhbGciOiJIUzI1NiJ9.eyJpc3MiOiJPbmxpbmUgSldUIEJ1aWxkZXIiLCJpYXQiOjE2MTg1MTc5NDMsImV4cCI6MTY1MDA1Mzk0MywiYXVkIjoiaHR0cHM6Ly9kYW5pZWxtYW5ndW0uY29tIiwic3ViIjoiZ2VvcmdlZGFuaWVsbWFuZ3VtQGdtYWlsLmNvbSIsIkpUSSI6Imhhc2hlZGRhbiJ9.zI42wXvwDHiATx9ycECz7JyATTn9P07wN-TRXvtCGcM",
					Remember: true,
				},
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			auth, profType, err := constructAuth(tc.args.username, tc.args.token, tc.args.password)
			if diff := cmp.Diff(tc.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nconstructAuth(...): -want error, +got error:\n%s", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.auth, auth); diff != "" {
				t.Errorf("\n%s\nconstructAuth(...): -want, +got:\n%s", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.pType, profType); diff != "" {
				t.Errorf("\n%s\nconstructAuth(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestParseID(t *testing.T) {
	type args struct {
		username string
		token    string
	}
	type want struct {
		id    string
		pType profile.TokenType
	}
	cases := map[string]struct {
		reason string
		args   args
		want   want
		err    error
	}{
		"ErrorInvalidToken": {
			reason: "If token is not a valid JWT an error should be returned.",
			args: args{
				token: "invalid",
			},
			err: jwt.NewValidationError("token contains an invalid number of segments", jwt.ValidationErrorMalformed),
		},
		"ErrorNoClaimID": {
			reason: "If token does not contain an ID an error should be returned.",
			args: args{
				token: "eyJ0eXAiOiJKV1QiLCJhbGciOiJIUzI1NiJ9.eyJpc3MiOiJPbmxpbmUgSldUIEJ1aWxkZXIiLCJpYXQiOjE2MTg1MTc1NDQsImV4cCI6MTY1MDA1MzU0NCwiYXVkIjoiaHR0cHM6Ly9kYW5pZWxtYW5ndW0uY29tIiwic3ViIjoiZ2VvcmdlZGFuaWVsbWFuZ3VtQGdtYWlsLmNvbSIsIkZpcnN0IjoiRGFuIiwiU3VybmFtZSI6Ik1hbmd1bSJ9.8F4mgY5-lpt2KmGx7Z8yeSorfs-WRgdJmCq8mCcrxZQ",
			},
			err: errors.New(errNoIDInToken),
		},
		"SuccessfulToken": {
			reason: "Providing a valid token should return a valid auth request.",
			args: args{
				token: "eyJ0eXAiOiJKV1QiLCJhbGciOiJIUzI1NiJ9.eyJpc3MiOiJPbmxpbmUgSldUIEJ1aWxkZXIiLCJpYXQiOjE2MTg1MTc5NDMsImV4cCI6MTY1MDA1Mzk0MywiYXVkIjoiaHR0cHM6Ly9kYW5pZWxtYW5ndW0uY29tIiwic3ViIjoiZ2VvcmdlZGFuaWVsbWFuZ3VtQGdtYWlsLmNvbSIsIkpUSSI6Imhhc2hlZGRhbiJ9.zI42wXvwDHiATx9ycECz7JyATTn9P07wN-TRXvtCGcM",
			},
			want: want{
				id:    "hasheddan",
				pType: profile.Token,
			},
		},
		"Successful": {
			reason: "Providing a username should return a valid auth request.",
			args: args{
				username: "cool-user",
			},
			want: want{
				id:    "cool-user",
				pType: profile.User,
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			id, pType, err := parseID(tc.args.username, tc.args.token)
			if diff := cmp.Diff(tc.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nparseID(...): -want error, +got error:\n%s", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.id, id, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nparseID(...): -want, +got:\n%s", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.pType, pType, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nparseID(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestExtractSession(t *testing.T) {
	errBoom := errors.New("boom")
	cook := http.Cookie{
		Name:  "SID",
		Value: "cool-session",
	}
	type args struct {
		res  *http.Response
		name string
	}
	cases := map[string]struct {
		reason string
		args   args
		want   string
		err    error
	}{
		"ErrorNoCookieFailReadBody": {
			reason: "Should return an error if cookie does not exist and we fail to read body.",
			args: args{
				res: &http.Response{
					Body: io.NopCloser(iotest.ErrReader(errBoom)),
				},
			},
			err: errors.Wrap(errBoom, errReadBody),
		},
		"ErrorNoCookie": {
			reason: "Should return an error if cookie does not exist.",
			args: args{
				res: &http.Response{
					Body: io.NopCloser(bytes.NewBuffer([]byte("unauthorized"))),
				},
			},
			err: errors.Errorf(errParseCookieFmt, "unauthorized"),
		},
		"Successful": {
			reason: "Should return cookie value if it exists.",
			args: args{
				res: &http.Response{
					Header: http.Header{"Set-Cookie": []string{cook.String()}},
				},
				name: cook.Name,
			},
			want: cook.Value,
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			session, err := extractSession(tc.args.res, tc.args.name)
			if diff := cmp.Diff(tc.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nextractSession(...): -want error, +got error:\n%s", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want, session); diff != "" {
				t.Errorf("\n%s\nextractSession(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestIsEmail(t *testing.T) {
	cases := map[string]struct {
		reason string
		user   string
		want   bool
	}{
		"UserIsEmail": {
			reason: "Should return true if username is an email address.",
			user:   "dan@upbound.io",
			want:   true,
		},
		"NotEmail": {
			reason: "Should return false if username is not an email address.",
			user:   "hasheddan",
			want:   false,
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := isEmail(tc.user)
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("\n%s\nisEmail(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}
