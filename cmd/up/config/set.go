package config

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"

	"github.com/pkg/errors"

	"github.com/upbound/up/internal/upbound"
)

const (
	errUpdateConfig  = "unable to update config file"
	errInvalidFile   = "invalid file format supplied. Must by JSON"
	errOnlyKVFileXOR = "only key and value OR file input is allowed"
)

type setCmd struct {
	Key   string `arg:"" optional:"" help:"Configuration Key."`
	Value string `arg:"" optional:"" help:"Configuration Value."`

	File *os.File `short:"f" help:"Configuration File. Must be in JSON format."`
}

// Run executes the set command.
func (c *setCmd) Run(upCtx *upbound.Context) error {
	if err := c.validateInput(); err != nil {
		return err
	}

	profile, _, err := upCtx.Cfg.GetDefaultUpboundProfile()
	if err != nil {
		return err
	}

	cfg := map[string]any{
		c.Key: c.Value,
	}
	if c.File != nil {
		cfg, err = c.mapFromFile()
		if err != nil {
			return err
		}
	}

	if err := c.addConfigs(upCtx, profile, cfg); err != nil {
		return err
	}
	return errors.Wrap(upCtx.CfgSrc.UpdateConfig(upCtx.Cfg), errUpdateConfig)
}

func (c *setCmd) validateInput() error {
	if c.Key != "" && c.Value != "" && c.File == nil {
		return nil
	}
	if c.Key == "" && c.Value == "" && c.File != nil {
		return nil
	}

	return errors.New(errOnlyKVFileXOR)
}

func (c *setCmd) mapFromFile() (map[string]any, error) {
	b, err := ioutil.ReadAll(c.File)
	if err != nil {
		return nil, err
	}

	var cfg map[string]any
	if err := json.Unmarshal(b, &cfg); err != nil {
		return nil, errors.Wrap(err, errInvalidFile)
	}

	return cfg, nil
}

func (c *setCmd) addConfigs(upCtx *upbound.Context, profile string, config map[string]any) error {
	for k, v := range config {
		if err := upCtx.Cfg.AddToBaseConfig(profile, k, fmt.Sprintf("%v", v)); err != nil {
			return err
		}
	}
	return nil
}
