package local

import (
	"context"

	"sigs.k8s.io/kind/pkg/cluster"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
)

// stopCmd destroys the local control plane.
type stopCmd struct{}

func (c *stopCmd) Run(ctx context.Context) error {
	provider := cluster.NewProvider()

	if err := provider.Delete(controlPlaneName, ""); err != nil {
		return errors.Wrap(err, "failed to delete the local control plane")
	}

	return nil
}
