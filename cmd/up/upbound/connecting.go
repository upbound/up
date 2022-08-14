package upbound

import (
	"fmt"

	"github.com/pterm/pterm"

	"github.com/upbound/up/internal/resources"
)

const (
	// TODO(tnthornton) replace these hardcode values with a query to Upbound
	// status field to derive the hostnames.
	hostNames = `upbound.%[1]s accounts.%[1]s static.%[1]s api.%[1]s proxy.%[1]s icons.%[1]s`
)

func outputConnectingInfo(ipAddress, hostNames string) {
	pterm.Println()
	pterm.Info.WithPrefix(eyesPrefix).Println("Next Steps 👇")
	pterm.Println()
	pterm.Println("👉 (1): Add the following entry to your /etc/hosts file:")
	pterm.Println()
	pterm.Printf("%s\t%s", ipAddress, fmt.Sprintf(hostNames, resources.Domain))
	pterm.Println()
	pterm.Println()
	pterm.Printfln("👉 (2): Go to http://upbound.%s in your browser.", resources.Domain)
}
