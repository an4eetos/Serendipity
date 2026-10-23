package usecase

import (
	"context"
	"fmt"
	"time"

	discussion "github.com/an4eetos/serendipity/backend/internal/discussion/domain"
	kernel "github.com/an4eetos/serendipity/backend/internal/kernel/domain"
	"github.com/an4eetos/serendipity/backend/internal/kernel/ports/portstest"
	"github.com/an4eetos/serendipity/backend/internal/moderation/domain"
	quote "github.com/an4eetos/serendipity/backend/internal/quote/domain"
)

var at = time.Date(2026, 10, 24, 12, 0, 0, 0, time.UTC)

// recorder logs the port calls a usecase makes, in order, marking those made
// inside a unit of work.
type recorder struct{ calls []string }

func (r *recorder) add(ctx context.Context, format string, args ...any) {
	call := fmt.Sprintf(format, args...)
	if portstest.InTx(ctx) {
		call += " (tx)"
	}
	r.calls = append(r.calls, call)
}

// fakeQuotes implements quoteports.QuoteModeration on a map.
type fakeQuotes struct {
	log  *recorder
	byID map[string]quote.Quote
}

func (f *fakeQuotes) ListAny(ctx context.Context, ids []string) ([]quote.Quote, error) {
	f.log.add(ctx, "quotes.ListAny %v", ids)
	out := []quote.Quote{}
	for _, id := range ids {
		if q, ok := f.byID[id]; ok {
			out = append(out, q)
		}
	}
	return out, nil
}

func (f *fakeQuotes) Remove(ctx context.Context, id, reason string) error {
	f.log.add(ctx, "quotes.Remove %s %q", id, reason)
	q, ok := f.byID[id]
	if !ok || q.Removed() {
		return kernel.ErrNotFound
	}
	q.RemovedAt, q.RemovedReason = &at, reason
	f.byID[id] = q
	return nil
}

func (f *fakeQuotes) Restore(ctx context.Context, id string, wordCap quote.CoverageCap) error {
	f.log.add(ctx, "quotes.Restore %s cap=%d", id, wordCap)
	q, ok := f.byID[id]
	if !ok || !q.Removed() {
		return kernel.ErrNotFound
	}
	q.RemovedAt, q.RemovedReason = nil, ""
	f.byID[id] = q
	return nil
}

// fakeConversations implements discussionports.ConversationModeration on a
// map.
type fakeConversations struct {
	log  *recorder
	byID map[string]discussion.Conversation
}

func (f *fakeConversations) ListAny(ctx context.Context, ids []string) ([]discussion.Conversation, error) {
	f.log.add(ctx, "conversations.ListAny %v", ids)
	out := []discussion.Conversation{}
	for _, id := range ids {
		if c, ok := f.byID[id]; ok {
			out = append(out, c)
		}
	}
	return out, nil
}

func (f *fakeConversations) Remove(ctx context.Context, id, reason string) error {
	f.log.add(ctx, "conversations.Remove %s %q", id, reason)
	c, ok := f.byID[id]
	if !ok || c.Removed() {
		return kernel.ErrNotFound
	}
	c.RemovedAt, c.RemovedReason = &at, reason
	f.byID[id] = c
	return nil
}

func (f *fakeConversations) Restore(ctx context.Context, id string) error {
	f.log.add(ctx, "conversations.Restore %s", id)
	c, ok := f.byID[id]
	if !ok || !c.Removed() {
		return kernel.ErrNotFound
	}
	c.RemovedAt, c.RemovedReason = nil, ""
	f.byID[id] = c
	return nil
}

type fakeReports struct {
	log       *recorder
	open      []domain.Report
	created   []domain.Report
	createErr error
	// resolved is how many open reports Resolve closes.
	resolved  int
	listLimit int
}

func (f *fakeReports) Create(ctx context.Context, r domain.Report) (domain.Report, error) {
	f.log.add(ctx, "reports.Create")
	if f.createErr != nil {
		return domain.Report{}, f.createErr
	}
	r.ID, r.CreatedAt = fmt.Sprintf("report-%d", len(f.created)+1), at
	f.created = append(f.created, r)
	return r, nil
}

func (f *fakeReports) ListOpen(ctx context.Context, limit int) ([]domain.Report, error) {
	f.log.add(ctx, "reports.ListOpen")
	f.listLimit = limit
	return f.open, nil
}

func (f *fakeReports) Resolve(ctx context.Context, target domain.TargetRef, status domain.ReportStatus, adminID string) (int, error) {
	f.log.add(ctx, "reports.Resolve %s/%s %s by %s", target.Kind, target.ID, status, adminID)
	return f.resolved, nil
}

type fakeTakedowns struct {
	log       *recorder
	byID      map[string]domain.Takedown
	active    []domain.Takedown
	createErr error
	listLimit int
}

func (f *fakeTakedowns) Create(ctx context.Context, t domain.Takedown) (domain.Takedown, error) {
	f.log.add(ctx, "takedowns.Create %s/%s %q", t.Target.Kind, t.Target.ID, t.Reason)
	if f.createErr != nil {
		return domain.Takedown{}, f.createErr
	}
	if f.byID == nil {
		f.byID = map[string]domain.Takedown{}
	}
	t.ID, t.CreatedAt = "takedown-new", at
	f.byID[t.ID] = t
	return t, nil
}

func (f *fakeTakedowns) Get(ctx context.Context, id string) (domain.Takedown, error) {
	f.log.add(ctx, "takedowns.Get %s", id)
	t, ok := f.byID[id]
	if !ok {
		return domain.Takedown{}, kernel.ErrNotFound
	}
	return t, nil
}

func (f *fakeTakedowns) ListActive(ctx context.Context, limit int) ([]domain.Takedown, error) {
	f.log.add(ctx, "takedowns.ListActive")
	f.listLimit = limit
	return f.active, nil
}

func (f *fakeTakedowns) MarkRestored(ctx context.Context, id, adminID string) (domain.Takedown, error) {
	f.log.add(ctx, "takedowns.MarkRestored %s by %s", id, adminID)
	t, ok := f.byID[id]
	if !ok || t.RestoredAt != nil {
		return domain.Takedown{}, domain.ErrAlreadyRestored
	}
	t.RestoredAt, t.RestoredBy = &at, &adminID
	f.byID[id] = t
	return t, nil
}

type fakeAdmins struct {
	admins map[string]bool
	err    error
	asked  []string
}

func (f *fakeAdmins) IsAdmin(_ context.Context, userID string) (bool, error) {
	f.asked = append(f.asked, userID)
	return f.admins[userID], f.err
}

// world is reportable content shared by the usecase tests:
//
//   - "q-public": ada's public quote, discussed in "c-answered" (answered)
//     and "c-silent" (never answered);
//   - "q-private": ada's private quote;
//   - "q-removed": ada's public quote, removed by a moderator, discussed in
//     "c-on-removed";
//   - "c-removed": a removed discussion of "q-public".
type world struct {
	log           *recorder
	quotes        *fakeQuotes
	conversations *fakeConversations
}

func newWorld() world {
	log := &recorder{}
	answered := []discussion.Message{{Role: discussion.RoleAssistant, Content: "Hm."}}
	public := quote.Quote{ID: "q-public", UserID: "ada", Visibility: quote.VisibilityPublic, Text: "To be"}
	private := quote.Quote{ID: "q-private", UserID: "ada", Visibility: quote.VisibilityPrivate}
	removed := quote.Quote{ID: "q-removed", UserID: "ada", Visibility: quote.VisibilityPublic, RemovedAt: &at, RemovedReason: "copyright"}
	return world{
		log: log,
		quotes: &fakeQuotes{log: log, byID: map[string]quote.Quote{
			public.ID: public, private.ID: private, removed.ID: removed,
		}},
		conversations: &fakeConversations{log: log, byID: map[string]discussion.Conversation{
			"c-answered":   {ID: "c-answered", UserID: "ada", QuoteID: "q-public", Messages: answered},
			"c-silent":     {ID: "c-silent", UserID: "ada", QuoteID: "q-public", Messages: []discussion.Message{}},
			"c-on-removed": {ID: "c-on-removed", UserID: "ada", QuoteID: "q-removed", Messages: answered},
			"c-removed":    {ID: "c-removed", UserID: "ada", QuoteID: "q-public", Messages: answered, RemovedAt: &at, RemovedReason: "spam"},
		}},
	}
}
