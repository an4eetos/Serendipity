package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/an4eetos/serendipity/backend/internal/catalog/domain"
)

type fakeCatalog struct {
	results []domain.SearchResult
	err     error
	queries []string
}

func (f *fakeCatalog) Search(_ context.Context, q string) ([]domain.SearchResult, error) {
	f.queries = append(f.queries, q)
	return f.results, f.err
}

func TestSearchWorks(t *testing.T) {
	t.Run("rejects a short query without searching", func(t *testing.T) {
		cat := &fakeCatalog{}
		_, err := NewSearchWorks(cat).Execute(t.Context(), " a ")
		if !errors.Is(err, domain.ErrQueryTooShort) {
			t.Fatalf("err = %v", err)
		}
		if len(cat.queries) != 0 {
			t.Errorf("searched %v", cat.queries)
		}
	})

	t.Run("reports an unavailable catalog", func(t *testing.T) {
		_, err := NewSearchWorks(&fakeCatalog{err: errors.New("timeout")}).Execute(t.Context(), "plato")
		if !errors.Is(err, domain.ErrSearchUnavailable) {
			t.Fatalf("err = %v", err)
		}
	})

	t.Run("trims the query and caps subjects", func(t *testing.T) {
		cat := &fakeCatalog{results: []domain.SearchResult{
			{Title: "Republic", Subjects: []string{"1", "2", "3", "4", "5", "6", "7", "8", "9"}},
		}}
		got, err := NewSearchWorks(cat).Execute(context.Background(), "  plato  ")
		if err != nil {
			t.Fatal(err)
		}
		if cat.queries[0] != "plato" {
			t.Errorf("query = %q", cat.queries[0])
		}
		if n := len(got[0].Subjects); n != domain.MaxSubjects {
			t.Errorf("kept %d subjects", n)
		}
	})
}
