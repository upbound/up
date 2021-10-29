package xpls

// Cmd --
type Cmd struct {
	Serve serveCmd `cmd:"" group:"xpls" help:"run a server for Crossplane definitions using the Language Server Protocol."`
}
