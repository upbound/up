package cloud

import (
	"testing"

	"github.com/crossplane/crossplane-runtime/pkg/test"
	"github.com/dgrijalva/jwt-go"
	"github.com/google/go-cmp/cmp"
	"github.com/pkg/errors"

	"github.com/upbound/up/internal/config"
)

func TestParseID(t *testing.T) {
	type args struct {
		username string
		token    string
	}
	type want struct {
		id    string
		pType config.ProfileType
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
				pType: config.TokenProfileType,
			},
		},
		"Successful": {
			reason: "Providing a username should return a valid auth request.",
			args: args{
				username: "cool-user",
			},
			want: want{
				id:    "cool-user",
				pType: config.UserProfileType,
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
