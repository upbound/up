package cloud

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"time"

	"github.com/alecthomas/kong"
	"github.com/dgrijalva/jwt-go"
	"github.com/pkg/errors"

	"github.com/upbound/up/internal/config"
)

const (
	defaultClientTimeout = 30 * time.Second
	defaultLoginURL      = "https://api.upbound.io/v1/login"
	cookieName           = "SID"

	errLoginFailed        = "unable to login"
	errParseCookie        = "unable to parse session cookie"
	errParseCookieFmt     = "unable to parse session cookie: %s"
	errNoUserOrToken      = "either username or token must be provided"
	errUsernameNoPassword = "username provided without password"
	errNoIDInToken        = "token is missing ID"
)

// loginCmd adds a user or token profile with session token to the up config
// file.
type loginCmd struct {
	Password string `short:"p" env:"UP_PASSWORD" help:"Password for specified user."`
}

// Run executes the login command.
func (c *loginCmd) Run(ctx *kong.Context, username User, token Token) error {
	// TODO(hasheddan): prompt for input if only username is supplied or
	// neither.
	auth, pType, err := constructAuth(string(username), c.Password, string(token))
	if err != nil {
		return errors.Wrap(err, errLoginFailed)
	}
	src, err := config.NewFSSource()
	if err != nil {
		return errors.Wrap(err, errLoginFailed)
	}
	conf, err := src.GetConfig()
	if err != nil {
		return errors.Wrap(err, errLoginFailed)
	}
	jsonStr, err := json.Marshal(auth)
	if err != nil {
		return errors.Wrap(err, errLoginFailed)
	}
	req, err := http.NewRequest(http.MethodPost, defaultLoginURL, bytes.NewReader(jsonStr))
	if err != nil {
		return errors.Wrap(err, errLoginFailed)
	}
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{
		Timeout: 10 * time.Second,
	}
	res, err := client.Do(req)
	if err != nil {
		return errors.Wrap(err, errLoginFailed)
	}
	defer res.Body.Close()
	session, err := extractSession(res, cookieName)
	if err != nil {
		return errors.Wrap(err, errLoginFailed)
	}
	if err := conf.AddOrUpdateCloudProfile(config.Profile{
		Type:       pType,
		Identifier: auth.ID,
		Session:    session,
	}); err != nil {
		return err
	}
	if len(conf.Cloud.Profiles) == 1 {
		if err := conf.SetDefaultCloudProfile(auth.ID); err != nil {
			return errors.Wrap(err, errLoginFailed)
		}
	}
	return src.UpdateConfig(conf)
}

// auth is the request body sent to authenticate a user or token.
type auth struct {
	ID       string `json:"id"`
	Password string `json:"password"`
	Remember bool   `json:"remember"`
}

// constructAuth constructs the body of an Upbound Cloud authentication request
// given the provided credentials.
func constructAuth(username string, password string, token string) (*auth, config.ProfileType, error) {
	var id string
	var pType config.ProfileType
	pass := password
	if username == "" && token == "" {
		return nil, pType, errors.New(errNoUserOrToken)
	}

	// NOTE(hasheddan): xor tag prevents both username and token being provided,
	// but we default to username flow if provided.
	if token != "" {
		p := jwt.Parser{}
		claims := &jwt.StandardClaims{}
		_, _, err := p.ParseUnverified(token, claims)
		if err != nil {
			return nil, pType, err
		}
		if claims.Id == "" {
			return nil, pType, errors.New(errNoIDInToken)
		}
		id = claims.Id
		pass = token
		pType = config.TokenProfileType
	}
	if username != "" {
		id = username
		if password == "" {
			return nil, pType, errors.New(errUsernameNoPassword)
		}
		pType = config.UserProfileType
	}
	return &auth{
		ID:       id,
		Password: pass,
		Remember: true,
	}, pType, nil
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
		return "", errors.Wrap(err, errParseCookie)
	}
	return "", errors.Errorf(errParseCookieFmt, string(b))
}
