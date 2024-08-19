package local

import (
	"context"

	"sigs.k8s.io/kind/pkg/cluster"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
)

// teardownCmd destroys the local control plane.
type teardownCmd struct{}

func (c *teardownCmd) Run(ctx context.Context) error {
	provider := cluster.NewProvider()

	if err := provider.Delete(controlPlaneName, ""); err != nil {
		return errors.Wrap(err, "failed to delete control plane")
	}

	return nil
}
