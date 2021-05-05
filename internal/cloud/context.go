package cloud

import (
	"net/http"
	"net/http/cookiejar"
	"net/url"

	"github.com/upbound/up-sdk-go"
	"github.com/upbound/up/internal/config"
)

const (
	// UserAgent is the default user agent to use to make requests to the
	// Upbound Cloud API.
	UserAgent = "up-cli"
	// CookieName is the default cookie name used to identify a session token.
	CookieName = "SID"
)

// Context includes common data that Upbound Cloud consumers may utilize.
type Context struct {
	Profile  string
	ID       string
	Token    string
	Type     config.ProfileType
	Account  string
	Endpoint *url.URL
	Cfg      *config.Config
	CfgSrc   config.Source
}

// ExtractConfig performs extraction of configuration from the default source,
// which is the ~/.up/config.json file on the local filesystem.
func ExtractConfig() (*config.Config, config.Source, error) {
	src, err := config.NewFSSource()
	if err != nil {
		return nil, nil, err
	}
	conf, err := src.GetConfig()
	if err != nil {
		return nil, nil, err
	}
	return conf, src, nil
}

// BuildSDKConfig builds an Upbound SDK config suitable for usage with any
// service client.
func BuildSDKConfig(session string, endpoint *url.URL) (*up.Config, error) {
	cj, err := cookiejar.New(nil)
	if err != nil {
		return nil, err
	}
	cj.SetCookies(endpoint, []*http.Cookie{{
		Name:  CookieName,
		Value: session,
	},
	})
	client := up.NewClient(func(c *up.HTTPClient) {
		c.BaseURL = endpoint
		c.HTTP = &http.Client{
			Jar: cj,
		}
		c.UserAgent = UserAgent
	})
	return up.NewConfig(func(c *up.Config) {
		c.Client = client
	}), nil
}
