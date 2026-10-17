package usecase

import (
	"context"
	"errors"
	"strings"
	"testing"

	catalog "github.com/an4eetos/serendipity/backend/internal/catalog/domain"
	kernel "github.com/an4eetos/serendipity/backend/internal/kernel/domain"
	"github.com/an4eetos/serendipity/backend/internal/kernel/ports/portstest"
	"github.com/an4eetos/serendipity/backend/internal/kindleimport/domain"
	library "github.com/an4eetos/serendipity/backend/internal/library/domain"
	libraryports "github.com/an4eetos/serendipity/backend/internal/library/ports"
	quote "github.com/an4eetos/serendipity/backend/internal/quote/domain"
	quoteports "github.com/an4eetos/serendipity/backend/internal/quote/ports"
)

type fakeWorks struct{ ensured []catalog.Work }

func (f *fakeWorks) Ensure(_ context.Context, w catalog.Work, _ string) (catalog.Work, error) {
	f.ensured = append(f.ensured, w)
	w.ID = "work-" + w.Title
	return w, nil
}

type fakeLibrary struct {
	libraryports.LibraryRepository
	byTitle map[string]library.Item
}

func (f *fakeLibrary) FindByTitle(_ context.Context, _, title string) (library.Item, error) {
	it, ok := f.byTitle[strings.ToLower(title)]
	if !ok {
		return library.Item{}, kernel.ErrNotFound
	}
	return it, nil
}

func (f *fakeLibrary) Add(_ context.Context, _, workID string) (library.Item, error) {
	it := library.Item{ID: "item-" + workID, Work: catalog.Work{ID: workID}}
	return it, nil
}

// fakeQuotes treats texts it has seen as duplicates.
type fakeQuotes struct {
	quoteports.QuoteRepository
	seen      map[string]bool
	drafts    []quote.Draft
	outsideTx bool
}

func (f *fakeQuotes) Import(ctx context.Context, _ string, d quote.Draft) (bool, error) {
	f.outsideTx = f.outsideTx || !portstest.InTx(ctx)
	if f.seen[d.Text] {
		return false, nil
	}
	f.seen[d.Text] = true
	f.drafts = append(f.drafts, d)
	return true, nil
}

func clipping(book, meta, text string) string {
	return book + "\n" + meta + "\n\n" + text + "\n==========\n"
}

func TestImportClippings(t *testing.T) {
	file := clipping("The Republic (Plato)", "- Your Highlight on page 12 | Location 170-172", "The beginning is the most important part of the work.") +
		clipping("The Republic (Plato)", "- Your Note on page 99 | Location 1500", "A note without a highlight.") +
		clipping("The Republic (Plato)", "- Your Highlight on page 13 | Location 180", strings.Repeat("word ", quote.MaxWords+1)) +
		clipping("Meditations (Marcus Aurelius)", "- Your Highlight at location 88-91", "You have power over your mind.") +
		clipping("Meditations (Marcus Aurelius)", "- Your Note at location 91", "Easy to say.")

	txm := &portstest.TxManager{}
	works := &fakeWorks{}
	lib := &fakeLibrary{byTitle: map[string]library.Item{
		"meditations": {ID: "item-med", Work: catalog.Work{ID: "work-med"}},
	}}
	quotes := &fakeQuotes{seen: map[string]bool{"You have power over your mind.": true}}

	report, err := NewImportClippings(txm, works, lib, quotes).Execute(t.Context(), "u1", strings.NewReader(file))
	if err != nil {
		t.Fatal(err)
	}
	want := domain.Report{Books: 2, Imported: 1, Duplicates: 1, SkippedTooLong: 1, OrphanNotes: 1}
	if report != want {
		t.Errorf("report = %+v, want %+v", report, want)
	}
	if len(works.ensured) != 1 || works.ensured[0].Title != "The Republic" || works.ensured[0].Authors[0] != "Plato" {
		t.Errorf("catalogued %#v; only the new book should be", works.ensured)
	}
	d := quotes.drafts[0]
	if d.WorkID != "work-The Republic" || *d.LibraryItemID != "item-work-The Republic" || d.Source != quote.SourceKindle || d.Location != "Page 12 · Loc 170-172" {
		t.Errorf("imported %#v", d)
	}
	if txm.Calls != 1 || quotes.outsideTx {
		t.Errorf("transactions = %d, writes outside one: %v; the import must be all or nothing", txm.Calls, quotes.outsideTx)
	}
}

func TestImportClippingsUnreadable(t *testing.T) {
	txm := &portstest.TxManager{}
	huge := clipping("Book (Author)", "- Your Highlight at location 1", strings.Repeat("a", 5<<20))
	_, err := NewImportClippings(txm, &fakeWorks{}, &fakeLibrary{}, &fakeQuotes{}).Execute(t.Context(), "u1", strings.NewReader(huge))
	if !errors.Is(err, domain.ErrUnreadable) || txm.Calls != 0 {
		t.Errorf("err = %v, transactions = %d", err, txm.Calls)
	}
}
