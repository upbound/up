package cloud

import (
	"context"

	sdkerrs "github.com/upbound/up-sdk-go/errors"
	"github.com/upbound/up-sdk-go/service/common"
	"github.com/upbound/up-sdk-go/service/configurations"
	"github.com/upbound/up-sdk-go/service/controlplanes"

	"github.com/upbound/up/internal/controlplane"
)

const (
	maxItems = 100

	notAvailable = "n/a"
)

// Client is the client used for interacting with the ControlPlanes API in
// Upbound Cloud.
type Client struct {
	ctp *controlplanes.Client
	cfg *configurations.Client

	account string
}

// New instantiates a new Client.
func New(ctp *controlplanes.Client, cfg *configurations.Client, account string) *Client {
	return &Client{
		ctp:     ctp,
		cfg:     cfg,
		account: account,
	}
}

// Get the ControlPlane corresponding to the given ControlPlane name.
func (c *Client) Get(ctx context.Context, name string) (*controlplane.Response, error) {
	resp, err := c.ctp.Get(context.Background(), c.account, name)

	if sdkerrs.IsNotFound(err) {
		return nil, controlplane.NewNotFound(err)
	}

	if err != nil {
		return nil, err
	}

	return convert(resp), nil
}

// List all ControlPlanes within the Space.
func (c *Client) List(ctx context.Context) ([]*controlplane.Response, error) {
	l, err := c.ctp.List(context.Background(), c.account, common.WithSize(maxItems))
	if err != nil {
		return nil, err
	}
	resps := []*controlplane.Response{}
	for _, r := range l.ControlPlanes {
		resps = append(resps, convert(&r))
	}
	return resps, nil
}

// Create a new ControlPlane with the given name and the supplied Options.
func (c *Client) Create(ctx context.Context, name string, opts controlplane.Options) (*controlplane.Response, error) {
	// Get the UUID from the Configuration name, if it exists.
	cfg, err := c.cfg.Get(context.Background(), c.account, opts.ConfigurationName)
	if err != nil {
		return nil, err
	}

	resp, err := c.ctp.Create(context.Background(), c.account, &controlplanes.ControlPlaneCreateParameters{
		Name:            name,
		Description:     opts.Description,
		ConfigurationID: cfg.ID,
	})
	if err != nil {
		return nil, err
	}

	return convert(resp), nil
}

// Delete the ControlPlane corresponding to the given ControlPlane name.
func (c *Client) Delete(ctx context.Context, name string) error {
	err := c.ctp.Delete(context.Background(), c.account, name)
	if sdkerrs.IsNotFound(err) {
		return controlplane.NewNotFound(err)
	}
	return err
}

func convert(ctp *controlplanes.ControlPlaneResponse) *controlplane.Response {

	var cfgName string
	var cfgStatus string
	// All Upbound managed control planes in an account should be associated to a configuration.
	// However, we should still list all control planes and indicate where this isn't the case.
	if ctp.ControlPlane.Configuration.Name != nil && ctp.ControlPlane.Configuration != EmptyControlPlaneConfiguration() {
		cfgName = *ctp.ControlPlane.Configuration.Name
		cfgStatus = string(ctp.ControlPlane.Configuration.Status)
	} else {
		cfgName, cfgStatus = notAvailable, notAvailable
	}

	return &controlplane.Response{
		ID:        ctp.ControlPlane.ID.String(),
		Name:      ctp.ControlPlane.Name,
		Status:    string(ctp.Status),
		Cfg:       cfgName,
		CfgStatus: cfgStatus,
	}
}

// EmptyControlPlaneConfiguration returns an empty ControlPlaneConfiguration with default values.
func EmptyControlPlaneConfiguration() controlplanes.ControlPlaneConfiguration {
	configuration := controlplanes.ControlPlaneConfiguration{}
	configuration.Status = controlplanes.ConfigurationInstallationQueued
	return configuration
}
