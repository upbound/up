package cloud

import (
	"bytes"
	"io"
	"net/http"
	"testing"
	"testing/iotest"

	"github.com/crossplane/crossplane-runtime/pkg/test"
	"github.com/dgrijalva/jwt-go"
	"github.com/google/go-cmp/cmp"
	"github.com/pkg/errors"

	"github.com/upbound/up/internal/config"
)

func TestConstructAuth(t *testing.T) {
	type args struct {
		username string
		password string
		token    string
	}
	type want struct {
		pType config.ProfileType
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
		"ErrorUsernameNoPassword": {
			reason: "Providing a username without a password should cause an error.",
			args: args{
				username: "cool-user",
			},
			err: errors.New(errUsernameNoPassword),
		},
		"SuccessfulToken": {
			reason: "Providing a valid token should return a valid auth request.",
			args: args{
				token: "eyJ0eXAiOiJKV1QiLCJhbGciOiJIUzI1NiJ9.eyJpc3MiOiJPbmxpbmUgSldUIEJ1aWxkZXIiLCJpYXQiOjE2MTg1MTc5NDMsImV4cCI6MTY1MDA1Mzk0MywiYXVkIjoiaHR0cHM6Ly9kYW5pZWxtYW5ndW0uY29tIiwic3ViIjoiZ2VvcmdlZGFuaWVsbWFuZ3VtQGdtYWlsLmNvbSIsIkpUSSI6Imhhc2hlZGRhbiJ9.zI42wXvwDHiATx9ycECz7JyATTn9P07wN-TRXvtCGcM",
			},
			want: want{
				pType: config.TokenProfileType,
				auth: &auth{
					ID:       "hasheddan",
					Password: "eyJ0eXAiOiJKV1QiLCJhbGciOiJIUzI1NiJ9.eyJpc3MiOiJPbmxpbmUgSldUIEJ1aWxkZXIiLCJpYXQiOjE2MTg1MTc5NDMsImV4cCI6MTY1MDA1Mzk0MywiYXVkIjoiaHR0cHM6Ly9kYW5pZWxtYW5ndW0uY29tIiwic3ViIjoiZ2VvcmdlZGFuaWVsbWFuZ3VtQGdtYWlsLmNvbSIsIkpUSSI6Imhhc2hlZGRhbiJ9.zI42wXvwDHiATx9ycECz7JyATTn9P07wN-TRXvtCGcM",
					Remember: true,
				},
			},
		},
		"SuccessfulUsername": {
			reason: "Providing a valid username and password should return a valid auth request.",
			args: args{
				username: "cool-user",
				password: "cool-password",
			},
			want: want{
				pType: config.UserProfileType,
				auth: &auth{
					ID:       "cool-user",
					Password: "cool-password",
					Remember: true,
				},
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			auth, pType, err := constructAuth(tc.args.username, tc.args.password, tc.args.token)
			if diff := cmp.Diff(tc.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nconstructAuth(...): -want error, +got error:\n%s", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.pType, pType); diff != "" {
				t.Errorf("\n%s\nconstructAuth(...): -want, +got:\n%s", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.auth, auth); diff != "" {
				t.Errorf("\n%s\nconstructAuth(...): -want, +got:\n%s", tc.reason, diff)
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
			err: errors.Wrap(errBoom, errParseCookie),
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
