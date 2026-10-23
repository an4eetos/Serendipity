package usecase

import (
	"context"
	"errors"
	"reflect"
	"testing"

	kernel "github.com/an4eetos/serendipity/backend/internal/kernel/domain"
	"github.com/an4eetos/serendipity/backend/internal/kernel/ports/portstest"
	"github.com/an4eetos/serendipity/backend/internal/moderation/domain"
	quote "github.com/an4eetos/serendipity/backend/internal/quote/domain"
)

func TestHideTarget(t *testing.T) {
	tests := []struct {
		name      string
		ref       domain.TargetRef
		reason    string
		createErr error
		wantErr   error
		wantCalls []string
	}{
		{"a quote", domain.TargetRef{Kind: domain.TargetQuote, ID: "q-public"}, "  Publisher's notice  ", nil, nil, []string{
			"quotes.ListAny [q-public]",
			`quotes.Remove q-public "Publisher's notice" (tx)`,
			`takedowns.Create quote/q-public "Publisher's notice" (tx)`,
			"reports.Resolve quote/q-public actioned by root (tx)",
		}},
		{"a discussion", domain.TargetRef{Kind: domain.TargetConversation, ID: "c-answered"}, "Harassment", nil, nil, []string{
			"conversations.ListAny [c-answered]",
			"quotes.ListAny [q-public]",
			`conversations.Remove c-answered "Harassment" (tx)`,
			`takedowns.Create conversation/c-answered "Harassment" (tx)`,
			"reports.Resolve conversation/c-answered actioned by root (tx)",
		}},
		{"already hidden", domain.TargetRef{Kind: domain.TargetQuote, ID: "q-removed"}, "again", nil, domain.ErrAlreadyRemoved,
			[]string{"quotes.ListAny [q-removed]"}},
		{"no reason for the owner", domain.TargetRef{Kind: domain.TargetQuote, ID: "q-public"}, "  ", nil, domain.ErrTakedownReason,
			[]string{"quotes.ListAny [q-public]"}},
		{"missing", domain.TargetRef{Kind: domain.TargetQuote, ID: "q-missing"}, "x", nil, kernel.ErrNotFound,
			[]string{"quotes.ListAny [q-missing]"}},
		{"unknown kind is refused before any lookup", domain.TargetRef{Kind: "comment", ID: "x"}, "x", nil, domain.ErrInvalidTarget, nil},
		{"another admin hid it first", domain.TargetRef{Kind: domain.TargetQuote, ID: "q-public"}, "x", domain.ErrAlreadyRemoved,
			domain.ErrAlreadyRemoved, []string{
				"quotes.ListAny [q-public]",
				`quotes.Remove q-public "x" (tx)`,
				`takedowns.Create quote/q-public "x" (tx)`,
			}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := newWorld()
			txm := &portstest.TxManager{}
			takedowns := &fakeTakedowns{log: w.log, createErr: tt.createErr}
			td, err := NewHideTarget(txm, &fakeReports{log: w.log}, takedowns, w.quotes, w.conversations).
				Execute(t.Context(), "root", tt.ref, tt.reason)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v, want %v", err, tt.wantErr)
			}
			if !reflect.DeepEqual(w.log.calls, tt.wantCalls) {
				t.Errorf("calls = %q, want %q", w.log.calls, tt.wantCalls)
			}
			if err != nil {
				return
			}
			if txm.Calls != 1 || td.ID == "" || td.Target != tt.ref || td.OwnerID != "ada" ||
				td.HiddenBy == nil || *td.HiddenBy != "root" {
				t.Errorf("Execute() = %#v, transactions %d", td, txm.Calls)
			}
		})
	}
}

// restoreWorld adds takedowns to the shared content.
func restoreWorld() (world, *fakeTakedowns) {
	w := newWorld()
	takedowns := &fakeTakedowns{log: w.log, byID: map[string]domain.Takedown{
		"td-quote":     {ID: "td-quote", Target: domain.TargetRef{Kind: domain.TargetQuote, ID: "q-removed"}, OwnerID: "ada"},
		"td-conv":      {ID: "td-conv", Target: domain.TargetRef{Kind: domain.TargetConversation, ID: "c-removed"}, OwnerID: "ada"},
		"td-quote-del": {ID: "td-quote-del", Target: domain.TargetRef{Kind: domain.TargetQuote, ID: "q-deleted"}, OwnerID: "ada"},
		"td-conv-del":  {ID: "td-conv-del", Target: domain.TargetRef{Kind: domain.TargetConversation, ID: "c-deleted"}, OwnerID: "ada"},
		"td-done":      {ID: "td-done", Target: domain.TargetRef{Kind: domain.TargetQuote, ID: "q-public"}, OwnerID: "ada", RestoredAt: &at},
	}}
	return w, takedowns
}

func TestRestoreTarget(t *testing.T) {
	const wordCap quote.CoverageCap = 777
	tests := []struct {
		name      string
		takedown  string
		wantErr   error
		wantCalls []string
	}{
		{"a quote, checked against the configured cap", "td-quote", nil, []string{
			"takedowns.Get td-quote",
			"quotes.ListAny [q-removed]",
			"quotes.Restore q-removed cap=777 (tx)",
			"takedowns.MarkRestored td-quote by root (tx)",
			"reports.Resolve quote/q-removed dismissed by root (tx)",
		}},
		{"a discussion", "td-conv", nil, []string{
			"takedowns.Get td-conv",
			"conversations.ListAny [c-removed]",
			"quotes.ListAny [q-public]",
			"conversations.Restore c-removed (tx)",
			"takedowns.MarkRestored td-conv by root (tx)",
			"reports.Resolve conversation/c-removed dismissed by root (tx)",
		}},
		// CR-001: the owner deleted the hidden content, so the takedown and
		// the open reports are closed with nothing to restore.
		{"a quote the owner deleted", "td-quote-del", nil, []string{
			"takedowns.Get td-quote-del",
			"quotes.ListAny [q-deleted]",
			"takedowns.MarkRestored td-quote-del by root (tx)",
			"reports.Resolve quote/q-deleted dismissed by root (tx)",
		}},
		{"a discussion the owner deleted", "td-conv-del", nil, []string{
			"takedowns.Get td-conv-del",
			"conversations.ListAny [c-deleted]",
			"takedowns.MarkRestored td-conv-del by root (tx)",
			"reports.Resolve conversation/c-deleted dismissed by root (tx)",
		}},
		{"already restored", "td-done", domain.ErrAlreadyRestored, []string{"takedowns.Get td-done"}},
		{"missing takedown", "td-missing", kernel.ErrNotFound, []string{"takedowns.Get td-missing"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w, takedowns := restoreWorld()
			txm := &portstest.TxManager{}
			td, err := NewRestoreTarget(txm, &fakeReports{log: w.log}, takedowns, w.quotes, w.conversations, wordCap).
				Execute(t.Context(), "root", tt.takedown)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v, want %v", err, tt.wantErr)
			}
			if !reflect.DeepEqual(w.log.calls, tt.wantCalls) {
				t.Errorf("calls = %q, want %q", w.log.calls, tt.wantCalls)
			}
			if err != nil {
				if txm.Calls != 0 {
					t.Errorf("transactions = %d, want none", txm.Calls)
				}
				return
			}
			if txm.Calls != 1 || td.RestoredAt == nil || td.RestoredBy == nil || *td.RestoredBy != "root" {
				t.Errorf("Execute() = %#v, transactions %d", td, txm.Calls)
			}
		})
	}

	t.Run("the restored quote is public again", func(t *testing.T) {
		w, takedowns := restoreWorld()
		_, err := NewRestoreTarget(&portstest.TxManager{}, &fakeReports{log: w.log}, takedowns, w.quotes, w.conversations, wordCap).
			Execute(t.Context(), "root", "td-quote")
		if q := w.quotes.byID["q-removed"]; err != nil || !q.IsPublic() || q.RemovedReason != "" {
			t.Errorf("quote after restore = %#v, %v", q, err)
		}
	})
}

// failingQuotes fails every lookup.
type failingQuotes struct {
	*fakeQuotes
	err error
}

func (f failingQuotes) ListAny(context.Context, []string) ([]quote.Quote, error) { return nil, f.err }

func TestRestoreTargetLookupFailure(t *testing.T) {
	w, takedowns := restoreWorld()
	errDB := errors.New("connection reset")
	txm := &portstest.TxManager{}
	_, err := NewRestoreTarget(txm, &fakeReports{log: w.log}, takedowns, failingQuotes{w.quotes, errDB}, w.conversations, 10).
		Execute(t.Context(), "root", "td-quote")
	// A failed lookup must not be taken for a deleted target: that would
	// close the takedown without restoring anything.
	if !errors.Is(err, errDB) || txm.Calls != 0 || takedowns.byID["td-quote"].RestoredAt != nil {
		t.Errorf("err = %v, transactions %d, takedown %#v", err, txm.Calls, takedowns.byID["td-quote"])
	}
}

func TestListTakedowns(t *testing.T) {
	w := newWorld()
	takedowns := &fakeTakedowns{log: w.log, active: []domain.Takedown{
		{ID: "td1", Target: domain.TargetRef{Kind: domain.TargetQuote, ID: "q-removed"}},
		{ID: "td2", Target: domain.TargetRef{Kind: domain.TargetConversation, ID: "c-removed"}},
		{ID: "td3", Target: domain.TargetRef{Kind: domain.TargetQuote, ID: "q-deleted"}},
	}}
	views, err := NewListTakedowns(takedowns, w.quotes, w.conversations).Execute(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	wantCalls := []string{
		"takedowns.ListActive",
		"conversations.ListAny [c-removed]",
		"quotes.ListAny [q-removed q-deleted q-public]",
	}
	if !reflect.DeepEqual(w.log.calls, wantCalls) || takedowns.listLimit != domain.MaxListed {
		t.Errorf("calls = %q, limit %d; want %q, %d", w.log.calls, takedowns.listLimit, wantCalls, domain.MaxListed)
	}
	if len(views) != 3 || views[0].Takedown.ID != "td1" || views[0].Content == nil || !views[0].Content.Removed ||
		views[1].Content == nil || views[1].Content.Conversation == nil || views[2].Content != nil {
		t.Errorf("Execute() = %+v; want content for td1 and td2, none for the deleted td3", views)
	}
}
