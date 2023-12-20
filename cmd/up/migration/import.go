package migration

import (
	"context"
	"fmt"
	"github.com/upbound/up/internal/migration"
)

type importCmd struct {
}

func (c *importCmd) Run(ctx context.Context, migCtx *migration.Context) error {
	fmt.Println("Importing ...")
	return nil
}
