package config

import (
	"encoding/json"
	"fmt"

	"github.com/upbound/up/internal/upbound"
)

type listCmd struct{}

// Run executes the list command.
func (c *listCmd) Run(upCtx *upbound.Context) error {
	profiles, err := upCtx.Cfg.GetUpboundProfiles()
	if err != nil {
		return err
	}

	b, err := json.MarshalIndent(profiles, "", "    ")
	if err != nil {
		return err
	}
	fmt.Println(string(b))
	return nil
}
