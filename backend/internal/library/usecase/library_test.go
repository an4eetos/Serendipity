package usecase

import (
	"context"
	"errors"
	"testing"
	"time"

	catalog "github.com/an4eetos/serendipity/backend/internal/catalog/domain"
	kernel "github.com/an4eetos/serendipity/backend/internal/kernel/domain"
	"github.com/an4eetos/serendipity/backend/internal/kernel/ports/portstest"
	"github.com/an4eetos/serendipity/backend/internal/library/domain"
	"github.com/an4eetos/serendipity/backend/internal/library/ports"
)

func ptr[T any](v T) *T { return &v }

type fakeWorks struct {
	ensured   []catalog.Work
	createdBy string
	outsideTx bool
}

func (f *fakeWorks) Ensure(ctx context.Context, w catalog.Work, createdBy string) (catalog.Work, error) {
	f.outsideTx = f.outsideTx || !portstest.InTx(ctx)
	f.ensured = append(f.ensured, w)
	f.createdBy = createdBy
	w.ID = "work-1"
	return w, nil
}

type fakeLibrary struct {
	ports.LibraryRepository
	items     map[string]domain.Item
	added     []string
	patches   []domain.Patch
	outsideTx bool
}

func (f *fakeLibrary) Add(ctx context.Context, _, workID string) (domain.Item, error) {
	f.outsideTx = f.outsideTx || !portstest.InTx(ctx)
	f.added = append(f.added, workID)
	return domain.Item{ID: "item-1", Work: catalog.Work{ID: workID}}, nil
}

func (f *fakeLibrary) Get(_ context.Context, _, id string) (domain.Item, error) {
	it, ok := f.items[id]
	if !ok {
		return domain.Item{}, kernel.ErrNotFound
	}
	return it, nil
}

func (f *fakeLibrary) Update(ctx context.Context, _, id string, p domain.Patch) (domain.Item, error) {
	f.outsideTx = f.outsideTx || !portstest.InTx(ctx)
	f.patches = append(f.patches, p)
	return domain.Item{ID: id}, nil
}

type fakeFiles struct {
	path string
	ttl  time.Duration
}

func (f *fakeFiles) SignedURL(_ context.Context, path string, ttl time.Duration) (string, error) {
	f.path, f.ttl = path, ttl
	return "https://signed/" + path, nil
}

func TestAddBook(t *testing.T) {
	t.Run("catalogues the work and adds it in one transaction", func(t *testing.T) {
		txm, works, lib := &portstest.TxManager{}, &fakeWorks{}, &fakeLibrary{}
		item, err := NewAddBook(txm, works, lib).Execute(t.Context(), "u1", catalog.Work{Title: "  Republic "})
		if err != nil {
			t.Fatal(err)
		}
		if item.Work.ID != "work-1" || works.ensured[0].Title != "Republic" || works.createdBy != "u1" {
			t.Errorf("item %#v, ensured %#v by %q", item, works.ensured, works.createdBy)
		}
		if txm.Calls != 1 || works.outsideTx || lib.outsideTx {
			t.Errorf("transactions = %d, writes outside one: %v %v", txm.Calls, works.outsideTx, lib.outsideTx)
		}
	})

	t.Run("rejects bad metadata before writing", func(t *testing.T) {
		works, lib := &fakeWorks{}, &fakeLibrary{}
		_, err := NewAddBook(&portstest.TxManager{}, works, lib).Execute(t.Context(), "u1", catalog.Work{Title: " "})
		if !errors.Is(err, catalog.ErrTitleRequired) || len(works.ensured) > 0 || len(lib.added) > 0 {
			t.Errorf("err = %v, ensured %d, added %d", err, len(works.ensured), len(lib.added))
		}
	})
}

func TestUpdateBook(t *testing.T) {
	lib := &fakeLibrary{}
	uc := NewUpdateBook(&portstest.TxManager{}, lib)
	if _, err := uc.Execute(t.Context(), "u1", "item-1", domain.Patch{Progress: ptr(2.0)}); !errors.Is(err, domain.ErrProgressRange) {
		t.Errorf("err = %v", err)
	}
	if len(lib.patches) > 0 {
		t.Fatal("an invalid patch must not be stored")
	}
	if _, err := uc.Execute(t.Context(), "u1", "item-1", domain.Patch{Progress: ptr(0.5)}); err != nil || len(lib.patches) != 1 || lib.outsideTx {
		t.Errorf("err = %v, patches %d, outside a transaction %v", err, len(lib.patches), lib.outsideTx)
	}
}

func TestGetFileLink(t *testing.T) {
	lib := &fakeLibrary{items: map[string]domain.Item{
		"with":    {ID: "with", FilePath: ptr("u1/a.pdf"), FileType: ptr(domain.FilePDF)},
		"without": {ID: "without"},
	}}
	files := &fakeFiles{}
	uc := NewGetFileLink(lib, files)

	link, err := uc.Execute(t.Context(), "u1", "with")
	if err != nil || link.URL != "https://signed/u1/a.pdf" || link.FileType != domain.FilePDF || files.ttl != domain.FileLinkTTL {
		t.Errorf("link = %#v, %v, ttl %v", link, err, files.ttl)
	}
	if _, err := uc.Execute(t.Context(), "u1", "without"); !errors.Is(err, domain.ErrNoFile) {
		t.Errorf("no file: err = %v", err)
	}
	if _, err := uc.Execute(t.Context(), "u1", "missing"); !errors.Is(err, kernel.ErrNotFound) {
		t.Errorf("missing item: err = %v", err)
	}
}
