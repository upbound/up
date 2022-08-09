package config

import (
	"encoding/json"
	"fmt"

	"github.com/upbound/up/internal/upbound"
)

type currentCmd struct{}

// Run executes the current command.
func (c *currentCmd) Run(upCtx *upbound.Context) error {
	_, profile, err := upCtx.Cfg.GetDefaultUpboundProfile()
	if err != nil {
		return err
	}

	b, err := json.MarshalIndent(profile, "", "    ")
	if err != nil {
		return err
	}
	fmt.Println(string(b))
	return nil
}
