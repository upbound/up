package migration

import (
	"context"
	"fmt"
	"github.com/upbound/up/internal/migration"
)

type exportCmd struct {
}

func (c *exportCmd) Run(ctx context.Context, migCtx *migration.Context) error {
	fmt.Println("Exporting ...")
	return nil
}
