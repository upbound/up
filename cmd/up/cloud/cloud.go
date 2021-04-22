package cloud

import (
	"net/url"

	"github.com/alecthomas/kong"
	"github.com/dgrijalva/jwt-go"
	"github.com/pkg/errors"

	"github.com/upbound/up/internal/cloud"
)

// AfterApply constructs and binds Upbound-specific context to any subcommands
// that have Run() methods that receive it.
func (c Cmd) AfterApply(ctx *kong.Context) error {
	var id string
	if c.Token != "" {
		p := jwt.Parser{}
		claims := &jwt.StandardClaims{}
		_, _, err := p.ParseUnverified(string(c.Token), claims)
		if err != nil {
			return err
		}
		if claims.Id == "" {
			return errors.New(errNoIDInToken)
		}
		id = claims.Id
	}
	if c.Username != "" {
		id = string(c.Username)
	}
	id, prof, conf, src, err := cloud.ExtractConfig(id)
	if err != nil {
		return err
	}
	org := id
	if prof.Org != "" {
		org = prof.Org
	}
	if c.Organization != "" {
		org = c.Organization
	}
	ctx.Bind(&cloud.Context{
		ID:       id,
		Type:     prof.Type,
		Org:      org,
		Session:  prof.Session,
		Endpoint: c.Endpoint,
		Cfg:      conf,
		CfgSrc:   src,
	})
	return nil
}

// Cmd contains commands for interacting with Upbound Cloud.
type Cmd struct {
	Login loginCmd `cmd:"" group:"cloud" help:"Login to Upbound Cloud."`

	ControlPlane controlPlaneCmd `cmd:"" name:"controlplane" aliases:"xp" group:"cloud" help:"Interact with control planes."`

	Endpoint     *url.URL `env:"UP_ENDPOINT" default:"https://api.upbound.io" help:"Endpoint used for Upbound API."`
	Username     string   `short:"u" env:"UP_USER" xor:"identifier" help:"Username used to execute command."`
	Token        string   `short:"t" env:"UP_TOKEN" xor:"identifier" help:"Token used to execute command."`
	Organization string   `short:"o" env:"UP_ORG" help:"Organization used to execute command."`
}
