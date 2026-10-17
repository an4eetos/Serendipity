package usecase

import (
	"context"
	"errors"
	"testing"

	catalog "github.com/an4eetos/serendipity/backend/internal/catalog/domain"
	kernel "github.com/an4eetos/serendipity/backend/internal/kernel/domain"
	"github.com/an4eetos/serendipity/backend/internal/kernel/ports/portstest"
	library "github.com/an4eetos/serendipity/backend/internal/library/domain"
	libraryports "github.com/an4eetos/serendipity/backend/internal/library/ports"
	"github.com/an4eetos/serendipity/backend/internal/quote/domain"
	"github.com/an4eetos/serendipity/backend/internal/quote/ports"
)

func ptr[T any](v T) *T { return &v }

type fakeLibrary struct {
	libraryports.LibraryRepository
	lookups int
}

func (f *fakeLibrary) Get(_ context.Context, _, id string) (library.Item, error) {
	f.lookups++
	if id != "item-1" {
		return library.Item{}, kernel.ErrNotFound
	}
	return library.Item{ID: "item-1", Work: catalog.Work{ID: "work-1"}}, nil
}

type fakeQuotes struct {
	ports.QuoteRepository
	stored    domain.Quote
	created   []domain.Draft
	patches   []domain.Patch
	deleted   []string
	outsideTx bool
}

func (f *fakeQuotes) write(ctx context.Context) {
	f.outsideTx = f.outsideTx || !portstest.InTx(ctx)
}

func (f *fakeQuotes) Create(ctx context.Context, _ string, d domain.Draft) (domain.Quote, error) {
	f.write(ctx)
	f.created = append(f.created, d)
	return domain.Quote{ID: "q1", WorkID: d.WorkID, Text: d.Text}, nil
}

func (f *fakeQuotes) Get(_ context.Context, _, id string) (domain.Quote, error) {
	if id != f.stored.ID {
		return domain.Quote{}, kernel.ErrNotFound
	}
	return f.stored, nil
}

func (f *fakeQuotes) Update(ctx context.Context, _, id string, p domain.Patch) (domain.Quote, error) {
	f.write(ctx)
	f.patches = append(f.patches, p)
	return f.stored, nil
}

func (f *fakeQuotes) Delete(ctx context.Context, _, id string) error {
	f.write(ctx)
	f.deleted = append(f.deleted, id)
	return nil
}

func TestCreateQuote(t *testing.T) {
	t.Run("cites the library item's work", func(t *testing.T) {
		lib, quotes := &fakeLibrary{}, &fakeQuotes{}
		_, err := NewCreateQuote(&portstest.TxManager{}, lib, quotes).
			Execute(t.Context(), "u1", "item-1", domain.Draft{Text: " To be ", Location: "Act 3"})
		if err != nil {
			t.Fatal(err)
		}
		d := quotes.created[0]
		if d.WorkID != "work-1" || d.LibraryItemID == nil || *d.LibraryItemID != "item-1" ||
			d.Text != "To be" || d.Source != domain.SourceUpload || quotes.outsideTx {
			t.Errorf("created %#v, outside a transaction %v", d, quotes.outsideTx)
		}
	})

	t.Run("checks the text before looking the book up", func(t *testing.T) {
		lib, quotes := &fakeLibrary{}, &fakeQuotes{}
		_, err := NewCreateQuote(&portstest.TxManager{}, lib, quotes).Execute(t.Context(), "u1", "item-1", domain.Draft{Text: " "})
		if !errors.Is(err, domain.ErrEmpty) || lib.lookups > 0 || len(quotes.created) > 0 {
			t.Errorf("err = %v, lookups %d, created %d", err, lib.lookups, len(quotes.created))
		}
	})

	t.Run("someone else's book", func(t *testing.T) {
		_, err := NewCreateQuote(&portstest.TxManager{}, &fakeLibrary{}, &fakeQuotes{}).
			Execute(t.Context(), "u1", "item-2", domain.Draft{Text: "x"})
		if !errors.Is(err, kernel.ErrNotFound) {
			t.Errorf("err = %v", err)
		}
	})
}

func TestUpdateQuote(t *testing.T) {
	uncited := domain.Quote{ID: "q1", Text: "To be", Reaction: "Mine", Visibility: domain.VisibilityPrivate}

	quotes := &fakeQuotes{stored: uncited}
	uc := NewUpdateQuote(&portstest.TxManager{}, quotes)
	if _, err := uc.Execute(t.Context(), "u1", "q1", domain.Patch{Visibility: ptr(domain.VisibilityPublic)}); !errors.Is(err, domain.ErrCitationMissing) {
		t.Errorf("publishing without a citation: err = %v", err)
	}
	if len(quotes.patches) > 0 {
		t.Fatal("a patch breaking the publishing rules must not be stored")
	}
	if _, err := uc.Execute(t.Context(), "u1", "q1", domain.Patch{Reaction: ptr("Changed")}); err != nil || len(quotes.patches) != 1 || quotes.outsideTx {
		t.Errorf("err = %v, patches %d, outside a transaction %v", err, len(quotes.patches), quotes.outsideTx)
	}
	if _, err := uc.Execute(t.Context(), "u1", "q2", domain.Patch{}); !errors.Is(err, kernel.ErrNotFound) {
		t.Errorf("missing quote: err = %v", err)
	}
}

func TestDeleteQuote(t *testing.T) {
	quotes := &fakeQuotes{}
	if err := NewDeleteQuote(&portstest.TxManager{}, quotes).Execute(t.Context(), "u1", "q1"); err != nil || len(quotes.deleted) != 1 || quotes.outsideTx {
		t.Errorf("err = %v, deleted %v, outside a transaction %v", err, quotes.deleted, quotes.outsideTx)
	}
}
