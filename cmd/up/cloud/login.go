package cloud

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"time"

	"github.com/alecthomas/kong"
	"github.com/pkg/errors"

	"github.com/upbound/up/internal/cloud"
	"github.com/upbound/up/internal/config"
)

const (
	defaultTimeout = 30 * time.Second
	loginPath      = "/v1/login"

	errLoginFailed    = "unable to login"
	errReadBody       = "unable to read response body"
	errParseCookieFmt = "unable to parse session cookie: %s"
	errNoUserOrToken  = "either username or token must be provided"
	errNoIDInToken    = "token is missing ID"
	errUpdateConfig   = "unable to update config file"
)

// loginCmd adds a user or token profile with session token to the up config
// file.
type loginCmd struct {
	Password string `short:"p" env:"UP_PASSWORD" help:"Password for specified user."`
}

// Run executes the login command.
func (c *loginCmd) Run(kong *kong.Context, cloudCtx *cloud.Context) error { // nolint:gocyclo
	// TODO(hasheddan): prompt for input if only username is supplied or
	// neither.
	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()
	auth, err := constructAuth(cloudCtx.ID, cloudCtx.Token, c.Password)
	if err != nil {
		return errors.Wrap(err, errLoginFailed)
	}
	jsonStr, err := json.Marshal(auth)
	if err != nil {
		return errors.Wrap(err, errLoginFailed)
	}
	cloudCtx.Endpoint.Path = loginPath
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, cloudCtx.Endpoint.String(), bytes.NewReader(jsonStr))
	if err != nil {
		return errors.Wrap(err, errLoginFailed)
	}
	req.Header.Set("Content-Type", "application/json")
	// NOTE(hasheddan): client timeout is handled with request context.
	client := &http.Client{}
	res, err := client.Do(req)
	if err != nil {
		return errors.Wrap(err, errLoginFailed)
	}
	defer res.Body.Close() // nolint:errcheck
	session, err := extractSession(res, cloud.CookieName)
	if err != nil {
		return errors.Wrap(err, errLoginFailed)
	}
	if err := cloudCtx.Cfg.AddOrUpdateCloudProfile(auth.ID, config.Profile{
		Type:    cloudCtx.Type,
		Session: session,
	}); err != nil {
		return err
	}
	if len(cloudCtx.Cfg.Cloud.Profiles) == 1 {
		if err := cloudCtx.Cfg.SetDefaultCloudProfile(auth.ID); err != nil {
			return errors.Wrap(err, errLoginFailed)
		}
	}
	return errors.Wrap(cloudCtx.CfgSrc.UpdateConfig(cloudCtx.Cfg), errUpdateConfig)
}

// auth is the request body sent to authenticate a user or token.
type auth struct {
	ID       string `json:"id"`
	Password string `json:"password"`
	Remember bool   `json:"remember"`
}

// constructAuth constructs the body of an Upbound Cloud authentication request
// given the provided credentials.
func constructAuth(id, token, password string) (*auth, error) {
	if id == "" {
		return nil, errors.New(errNoUserOrToken)
	}
	if password == "" {
		password = token
	}
	return &auth{
		ID:       id,
		Password: password,
		Remember: true,
	}, nil
}

// extractSession extracts the specified cookie from an HTTP response. The
// caller is responsible for closing the response body.
func extractSession(res *http.Response, cookieName string) (string, error) {
	for _, cook := range res.Cookies() {
		if cook.Name == cookieName {
			return cook.Value, nil
		}
	}
	b, err := io.ReadAll(res.Body)
	if err != nil {
		return "", errors.Wrap(err, errReadBody)
	}
	return "", errors.Errorf(errParseCookieFmt, string(b))
}
