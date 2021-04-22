package cloud

import (
	"bytes"
	"io"
	"net/http"
	"testing"
	"testing/iotest"

	"github.com/crossplane/crossplane-runtime/pkg/test"
	"github.com/google/go-cmp/cmp"
	"github.com/pkg/errors"

	"github.com/upbound/up/internal/config"
)

func TestConstructAuth(t *testing.T) {
	type args struct {
		id       string
		password string
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
		"Successful": {
			reason: "Providing a valid id and password should return a valid auth request.",
			args: args{
				id:       "cool-user",
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
			auth, err := constructAuth(tc.args.id, tc.args.password)
			if diff := cmp.Diff(tc.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nconstructAuth(...): -want error, +got error:\n%s", tc.reason, diff)
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
