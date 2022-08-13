package upbound

import "github.com/pterm/pterm"

const (
	ipAddress = "127.0.0.1"
	// TODO(tnthornton) replace these hardcode values with a query to Upbound
	// status field to derive the hostnames.
	hostNames = `upbound.local.upbound.io accounts.local.upbound.io static.local.upbound.io api.local.upbound.io static.local.upbound.io api.local.upbound.io proxy.local.upbound.io icons.local.upbound.io`
)

func outputConnectingInfo(ipAddress, hostNames string) {
	pterm.Println()
	pterm.Info.WithPrefix(eyesPrefix).Println("Next Steps 👇")
	pterm.Println()
	pterm.Println("👉 (1): Add the following entry to your /etc/hosts file:")
	pterm.Println()
	pterm.Printf("%s\t%s", ipAddress, hostNames)
	pterm.Println()
	pterm.Println()
	pterm.Println("👉 (2): Go to http://upbound.local.upbound.io in your browser.")
}
