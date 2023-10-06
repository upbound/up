package space

import (
	"context"

	xpcommonv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/dynamic"

	"github.com/upbound/up/internal/controlplane"
	"github.com/upbound/up/internal/resources"
)

var (
	resource = resources.ControlPlaneGVK.GroupVersion().WithResource("controlplanes")
)

// Client is the client used for interacting with the ControlPlanes API in an
// Upbound Space.
type Client struct {
	c dynamic.Interface
}

// New instantiates a new Client.
func New(c dynamic.Interface) *Client {
	return &Client{
		c: c,
	}
}

// Get the ControlPlane corresponding to the given ControlPlane name.
func (c *Client) Get(ctx context.Context, name string) (*resources.ControlPlane, error) {
	u, err := c.c.
		Resource(resource).
		Get(
			ctx,
			name,
			metav1.GetOptions{},
		)

	return &resources.ControlPlane{Unstructured: *u}, err
}

// List all ControlPlanes within the Space.
func (c *Client) List(ctx context.Context) (*resources.ControlPlaneList, error) {
	u, err := c.c.
		Resource(resource).
		List(
			ctx,
			metav1.ListOptions{},
		)
	return &resources.ControlPlaneList{UnstructuredList: *u}, err
}

// Create a new ControlPlane with the given name and the supplied Options.
func (c *Client) Create(ctx context.Context, name string, opts controlplane.Options) (*resources.ControlPlane, error) {
	ctp := &resources.ControlPlane{}
	ctp.SetName(name)
	ctp.SetGroupVersionKind(resources.ControlPlaneGVK)
	ctp.SetWriteConnectionSecretToReference(&xpcommonv1.SecretReference{
		Name:      opts.SecretName,
		Namespace: opts.SecretNamespace,
	})

	u, err := c.c.
		Resource(resource).
		Create(
			ctx,
			ctp.GetUnstructured(),
			metav1.CreateOptions{},
		)
	return &resources.ControlPlane{Unstructured: *u}, err
}

// Delete the ControlPlane corresponding to the given ControlPlane name.
func (c *Client) Delete(ctx context.Context, name string) error {
	return c.c.
		Resource(resource).
		Delete(
			context.Background(),
			name,
			metav1.DeleteOptions{},
		)
}
