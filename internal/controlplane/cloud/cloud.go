package cloud

import (
	"context"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/upbound/up-sdk-go/service/common"
	"github.com/upbound/up-sdk-go/service/configurations"
	"github.com/upbound/up-sdk-go/service/controlplanes"

	"github.com/upbound/up/internal/controlplane"
	"github.com/upbound/up/internal/resources"
)

const (
	maxItems = 100
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
func (c *Client) Get(ctx context.Context, name string) (*resources.ControlPlane, error) {
	resp, err := c.ctp.Get(context.Background(), c.account, name)
	if err != nil {
		return nil, err
	}

	ctp := &resources.ControlPlane{}
	ctp.SetName(resp.ControlPlane.Name)
	return ctp, nil
}

// List all ControlPlanes within the Space.
func (c *Client) List(ctx context.Context) (*resources.ControlPlaneList, error) {
	l, err := c.ctp.List(context.Background(), c.account, common.WithSize(maxItems))
	if err != nil {
		return nil, err
	}
	list := []unstructured.Unstructured{}
	for _, uc := range l.ControlPlanes {
		ctp := &resources.ControlPlane{}
		ctp.SetName(uc.ControlPlane.Name)
		list = append(list, *ctp.GetUnstructured())
	}
	return &resources.ControlPlaneList{
		UnstructuredList: unstructured.UnstructuredList{
			Items: list,
		},
	}, nil
}

// Create a new ControlPlane with the given name and the supplied Options.
func (c *Client) Create(ctx context.Context, name string, opts controlplane.Options) (*resources.ControlPlane, error) {
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

	ctp := &resources.ControlPlane{}
	ctp.SetName(resp.ControlPlane.Name)
	return ctp, nil
}

// Delete the ControlPlane corresponding to the given ControlPlane name.
func (c *Client) Delete(ctx context.Context, name string) error {
	return c.ctp.Delete(context.Background(), c.account, name)
}
