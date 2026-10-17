// Package usecase holds the catalog's business actions.
package usecase

import (
	"context"
	"errors"

	"github.com/an4eetos/serendipity/backend/internal/catalog/domain"
	"github.com/an4eetos/serendipity/backend/internal/catalog/ports"
)

// SearchWorks looks books up in the external catalog.
type SearchWorks struct {
	catalog ports.Catalog
}

func NewSearchWorks(catalog ports.Catalog) *SearchWorks {
	return &SearchWorks{catalog: catalog}
}

func (uc *SearchWorks) Execute(ctx context.Context, query string) ([]domain.SearchResult, error) {
	q, err := domain.SearchQuery(query)
	if err != nil {
		return nil, err
	}
	results, err := uc.catalog.Search(ctx, q)
	if err != nil {
		return nil, errors.Join(domain.ErrSearchUnavailable, err)
	}
	for i := range results {
		results[i].Subjects = domain.CapSubjects(results[i].Subjects)
	}
	return results, nil
}
