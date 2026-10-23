package usecase

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

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

// recorder logs the port calls a usecase makes, marking those made inside a
// unit of work.
type recorder struct{ calls []string }

func (r *recorder) add(ctx context.Context, call string) {
	if r == nil {
		return
	}
	if portstest.InTx(ctx) {
		call += " (tx)"
	}
	r.calls = append(r.calls, call)
}

type fakeQuotes struct {
	ports.QuoteRepository
	log       *recorder
	stored    domain.Quote
	created   []domain.Draft
	patches   []domain.Patch
	deleted   []string
	outsideTx bool
	// returned, when set, is what Update returns instead of stored: the row
	// as the database's coverage trigger left it.
	returned *domain.Quote
}

func (f *fakeQuotes) write(ctx context.Context) {
	f.outsideTx = f.outsideTx || !portstest.InTx(ctx)
}

func (f *fakeQuotes) Create(ctx context.Context, _ string, d domain.Draft) (domain.Quote, error) {
	f.write(ctx)
	f.created = append(f.created, d)
	return domain.Quote{ID: "q1", WorkID: d.WorkID, Text: d.Text}, nil
}

func (f *fakeQuotes) Get(ctx context.Context, _, id string) (domain.Quote, error) {
	f.log.add(ctx, "get")
	if id != f.stored.ID {
		return domain.Quote{}, kernel.ErrNotFound
	}
	return f.stored, nil
}

func (f *fakeQuotes) Update(ctx context.Context, _, id string, p domain.Patch) (domain.Quote, error) {
	f.write(ctx)
	f.log.add(ctx, "update")
	f.patches = append(f.patches, p)
	if f.returned != nil {
		return *f.returned, nil
	}
	return f.stored, nil
}

type fakeCoverage struct {
	log      *recorder
	coverage domain.Coverage
	err      error
	caps     []domain.CoverageCap
	locked   []string
	measured [][2]string
}

func (f *fakeCoverage) SetCap(ctx context.Context, wordCap domain.CoverageCap) error {
	f.log.add(ctx, "set cap")
	f.caps = append(f.caps, wordCap)
	return nil
}

func (f *fakeCoverage) LockWork(ctx context.Context, workID string) error {
	f.log.add(ctx, "lock work")
	f.locked = append(f.locked, workID)
	return nil
}

func (f *fakeCoverage) Coverage(ctx context.Context, workID, quoteID string) (domain.Coverage, error) {
	f.log.add(ctx, "coverage")
	f.measured = append(f.measured, [2]string{workID, quoteID})
	return f.coverage, f.err
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
	coverage := &fakeCoverage{}
	uc := NewUpdateQuote(&portstest.TxManager{}, quotes, coverage, 2500)
	if _, err := uc.Execute(t.Context(), "u1", "q1", domain.Patch{Visibility: ptr(domain.VisibilityPublic)}); !errors.Is(err, domain.ErrCitationMissing) {
		t.Errorf("publishing without a citation: err = %v", err)
	}
	if len(quotes.patches) > 0 || len(coverage.caps) > 0 {
		t.Fatal("a patch breaking the publishing rules must not be stored, nor coverage measured")
	}
	if _, err := uc.Execute(t.Context(), "u1", "q1", domain.Patch{Reaction: ptr("Changed")}); err != nil || len(quotes.patches) != 1 || quotes.outsideTx {
		t.Errorf("err = %v, patches %d, outside a transaction %v", err, len(quotes.patches), quotes.outsideTx)
	}
	txm := &portstest.TxManager{}
	if _, err := NewUpdateQuote(txm, quotes, coverage, 2500).Execute(t.Context(), "u1", "q2", domain.Patch{}); !errors.Is(err, kernel.ErrNotFound) || txm.Calls != 0 {
		t.Errorf("missing quote: err = %v, transactions %d; want it looked up before one", err, txm.Calls)
	}
}

// publishCalls is what UpdateQuote must do to publish: look the quote up
// before the transaction, then, inside it, read it again, give the database
// the cap, lock the work and measure coverage before writing.
var publishCalls = []string{"get", "get (tx)", "set cap (tx)", "lock work (tx)", "coverage (tx)", "update (tx)"}

func TestUpdateQuotePublishing(t *testing.T) {
	const wordCap = 10
	private := domain.Quote{ID: "q1", WorkID: "work-1", Text: "To be", Location: "Act 3", Reaction: "Mine",
		Visibility: domain.VisibilityPrivate, PassageWords: 2}
	public := private
	public.Visibility = domain.VisibilityPublic
	removed := private
	removed.RemovedAt, removed.RemovedReason = ptr(time.Now()), "copyright"
	publish := domain.Patch{Visibility: ptr(domain.VisibilityPublic)}

	tests := []struct {
		name       string
		stored     domain.Quote
		patch      domain.Patch
		coverage   domain.Coverage
		wantErr    error
		wantCalls  []string
		wantHidden *bool // the PassageHidden UpdateQuote stores; nil leaves it as it is
	}{
		{"shows a passage that fits under the cap", private, publish, domain.Coverage{Words: 8}, nil, publishCalls, ptr(false)},
		{"collapses a passage past the cap", private, publish, domain.Coverage{Words: 9}, nil, publishCalls, ptr(true)},
		{"shows a passage another quote already shows", private, publish,
			domain.Coverage{Words: 100, IdenticalShown: true}, nil, publishCalls, ptr(false)},
		{"the work's own cap raises the default", private, publish,
			domain.Coverage{Words: 9, CapOverride: ptr(20)}, nil, publishCalls, ptr(false)},
		{"the work's own cap lowers the default", private, publish,
			domain.Coverage{CapOverride: ptr(1)}, nil, publishCalls, ptr(true)},
		{"the reader can't keep the passage shown", private,
			domain.Patch{Visibility: ptr(domain.VisibilityPublic), PassageHidden: ptr(false)},
			domain.Coverage{Words: 9}, nil, publishCalls, ptr(true)},
		{"editing the reaction measures nothing", private, domain.Patch{Reaction: ptr("Changed")},
			domain.Coverage{}, nil, []string{"get", "get (tx)", "update (tx)"}, nil},
		{"the reader can't hide the passage either", public,
			domain.Patch{Reaction: ptr("Changed"), PassageHidden: ptr(true)},
			domain.Coverage{}, nil, []string{"get", "get (tx)", "update (tx)"}, nil},
		{"a quote that is already public isn't measured again", public, publish,
			domain.Coverage{Words: 100}, nil, []string{"get", "get (tx)", "update (tx)"}, nil},
		{"a removed quote can't be published", removed, publish,
			domain.Coverage{}, domain.ErrRemoved, []string{"get", "get (tx)"}, nil},
		{"a removed quote can still be edited", removed, domain.Patch{Reaction: ptr("Changed")},
			domain.Coverage{}, nil, []string{"get", "get (tx)", "update (tx)"}, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			log := &recorder{}
			quotes := &fakeQuotes{log: log, stored: tt.stored}
			coverage := &fakeCoverage{log: log, coverage: tt.coverage}
			_, err := NewUpdateQuote(&portstest.TxManager{}, quotes, coverage, wordCap).
				Execute(t.Context(), "u1", "q1", tt.patch)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v, want %v", err, tt.wantErr)
			}
			if !reflect.DeepEqual(log.calls, tt.wantCalls) {
				t.Errorf("calls = %q, want %q", log.calls, tt.wantCalls)
			}
			if tt.wantErr != nil {
				if len(quotes.patches) > 0 {
					t.Errorf("stored %#v; a refused patch must not be stored", quotes.patches)
				}
				return
			}
			if got := quotes.patches[0].PassageHidden; !reflect.DeepEqual(got, tt.wantHidden) {
				t.Errorf("stored PassageHidden = %v, want %v", deref(got), deref(tt.wantHidden))
			}
			if len(coverage.caps) > 0 {
				wantCaps, wantLocked, wantMeasured := []domain.CoverageCap{wordCap}, []string{"work-1"}, [][2]string{{"work-1", "q1"}}
				if !reflect.DeepEqual(coverage.caps, wantCaps) || !reflect.DeepEqual(coverage.locked, wantLocked) ||
					!reflect.DeepEqual(coverage.measured, wantMeasured) {
					t.Errorf("cap %v, locked %v, measured %v; want %v, %v, %v",
						coverage.caps, coverage.locked, coverage.measured, wantCaps, wantLocked, wantMeasured)
				}
			}
		})
	}
}

func TestUpdateQuoteCoverageFailure(t *testing.T) {
	errDB := errors.New("connection reset")
	stored := domain.Quote{ID: "q1", WorkID: "work-1", Text: "To be", Location: "Act 3", Reaction: "Mine", Visibility: domain.VisibilityPrivate}
	quotes := &fakeQuotes{stored: stored}
	_, err := NewUpdateQuote(&portstest.TxManager{}, quotes, &fakeCoverage{err: errDB}, 10).
		Execute(t.Context(), "u1", "q1", domain.Patch{Visibility: ptr(domain.VisibilityPublic)})
	if !errors.Is(err, errDB) || len(quotes.patches) > 0 {
		t.Errorf("err = %v, patches %d; want the failure returned and nothing stored", err, len(quotes.patches))
	}
}

func TestUpdateQuoteReturnsTheStoredRow(t *testing.T) {
	stored := domain.Quote{ID: "q1", WorkID: "work-1", Text: "To be", Location: "Act 3", Reaction: "Mine", Visibility: domain.VisibilityPrivate}
	// The trigger saw a concurrent publish this transaction's check didn't,
	// and collapsed the passage.
	collapsed := stored
	collapsed.Visibility, collapsed.PassageHidden = domain.VisibilityPublic, true
	quotes := &fakeQuotes{stored: stored, returned: &collapsed}
	q, err := NewUpdateQuote(&portstest.TxManager{}, quotes, &fakeCoverage{}, 2500).
		Execute(t.Context(), "u1", "q1", domain.Patch{Visibility: ptr(domain.VisibilityPublic)})
	if err != nil || !q.PassageHidden || *quotes.patches[0].PassageHidden {
		t.Errorf("Execute() = %#v, %v; want the stored row, collapsed by the database", q, err)
	}
}

func deref(b *bool) any {
	if b == nil {
		return nil
	}
	return *b
}

func TestDeleteQuote(t *testing.T) {
	quotes := &fakeQuotes{}
	if err := NewDeleteQuote(&portstest.TxManager{}, quotes).Execute(t.Context(), "u1", "q1"); err != nil || len(quotes.deleted) != 1 || quotes.outsideTx {
		t.Errorf("err = %v, deleted %v, outside a transaction %v", err, quotes.deleted, quotes.outsideTx)
	}
}
