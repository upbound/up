package cloud

import (
	"net/http"
	"net/http/cookiejar"
	"net/url"

	"github.com/upbound/up-sdk-go"
	"github.com/upbound/up/internal/config"
)

const (
	UserAgent  = "up-cli"
	CookieName = "SID"
)

type Context struct {
	ID       string
	Type     config.ProfileType
	Org      string
	Session  string
	Endpoint *url.URL
	Cfg      *config.Config
	CfgSrc   config.Source
}

func ExtractConfig(user string) (string, config.Profile, *config.Config, config.Source, error) {
	var profile config.Profile
	var id string
	src, err := config.NewFSSource()
	if err != nil {
		return id, profile, nil, nil, err
	}
	conf, err := src.GetConfig()
	if err != nil {
		return id, profile, nil, nil, err
	}
	if user == "" {
		id, profile, err = conf.GetDefaultCloudProfile()
		if err != nil {
			return id, profile, nil, nil, err
		}
	}
	return id, profile, conf, src, nil
}

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
