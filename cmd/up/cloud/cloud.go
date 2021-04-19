package cloud

import (
	"github.com/alecthomas/kong"
)

// User and Token are identified as unique types to allow them to be bound and
// received by subcommands.
type (
	// User is an Upbound Cloud username or email.
	User string
	// Token is an Upbound Cloud access token.
	Token string
)

// AfterApply binds global cloud flags to any subcommands that have Run()
// methods that receive the specified types.
func (c Cmd) AfterApply(ctx *kong.Context) error {
	ctx.Bind(c.Username)
	ctx.Bind(c.Token)
	return nil
}

// Cmd contains commands for interacting with Upbound Cloud.
type Cmd struct {
	Login loginCmd `cmd:"" help:"Login to Upbound Cloud."`

	Username User  `short:"u" env:"UP_USER" xor:"identifier" help:"Username used to execute command."`
	Token    Token `short:"t" env:"UP_TOKEN" xor:"identifier" help:"Token used to execute command."`
}
