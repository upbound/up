package export

import (
	"context"
	"github.com/crossplane/crossplane-runtime/pkg/errors"
)

type ResourceExporter interface {
	ExportResources(ctx context.Context) error
}

type UnstructuredExporter struct {
	fetcher   ResourceFetcher
	persister ResourcePersister
}

func NewUnstructuredExporter(f ResourceFetcher, p ResourcePersister) *UnstructuredExporter {
	return &UnstructuredExporter{
		fetcher:   f,
		persister: p,
	}
}

func (e *UnstructuredExporter) ExportResources(ctx context.Context) error {
	resources, err := e.fetcher.FetchResources(ctx)
	if err != nil {
		return errors.Wrap(err, "cannot fetch resources")
	}

	if err = e.persister.PersistResources(ctx, resources); err != nil {
		return errors.Wrap(err, "cannot persist resources")
	}

	return nil
}
