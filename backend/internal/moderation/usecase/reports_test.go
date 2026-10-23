package usecase

import (
	"errors"
	"reflect"
	"testing"

	kernel "github.com/an4eetos/serendipity/backend/internal/kernel/domain"
	"github.com/an4eetos/serendipity/backend/internal/kernel/ports/portstest"
	"github.com/an4eetos/serendipity/backend/internal/moderation/domain"
)

func TestCheckAdmin(t *testing.T) {
	errDB := errors.New("connection reset")
	tests := []struct {
		name      string
		admins    *fakeAdmins
		user      string
		want      error
		wantAsked int
	}{
		{"admin", &fakeAdmins{admins: map[string]bool{"root": true}}, "root", nil, 1},
		{"reader", &fakeAdmins{admins: map[string]bool{"root": true}}, "bo", domain.ErrNotAdmin, 1},
		{"no reader", &fakeAdmins{admins: map[string]bool{"": true}}, "", domain.ErrNotAdmin, 0},
		{"lookup fails", &fakeAdmins{err: errDB}, "root", errDB, 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := NewCheckAdmin(tt.admins).Execute(t.Context(), tt.user)
			if !errors.Is(err, tt.want) || len(tt.admins.asked) != tt.wantAsked {
				t.Errorf("err = %v, lookups %d; want %v, %d", err, len(tt.admins.asked), tt.want, tt.wantAsked)
			}
		})
	}
	var de kernel.DomainError
	if !errors.As(domain.ErrNotAdmin, &de) || de.Kind() != kernel.KindForbidden {
		t.Error("ErrNotAdmin must be Forbidden (403)")
	}
}

func TestCreateReport(t *testing.T) {
	quoteRef := func(id string) domain.TargetRef { return domain.TargetRef{Kind: domain.TargetQuote, ID: id} }
	convRef := func(id string) domain.TargetRef { return domain.TargetRef{Kind: domain.TargetConversation, ID: id} }
	tests := []struct {
		name      string
		reporter  string
		ref       domain.TargetRef
		reason    domain.Reason
		createErr error
		wantErr   error
		wantCalls []string
	}{
		{"someone else's public quote", "bo", quoteRef("q-public"), domain.ReasonCopyright, nil, nil,
			[]string{"quotes.ListAny [q-public]", "reports.Create (tx)"}},
		{"someone else's answered discussion", "bo", convRef("c-answered"), domain.ReasonHarassment, nil, nil,
			[]string{"conversations.ListAny [c-answered]", "quotes.ListAny [q-public]", "reports.Create (tx)"}},
		{"own quote", "ada", quoteRef("q-public"), domain.ReasonSpam, nil, domain.ErrOwnContent,
			[]string{"quotes.ListAny [q-public]"}},
		{"private quote", "bo", quoteRef("q-private"), domain.ReasonSpam, nil, kernel.ErrNotFound,
			[]string{"quotes.ListAny [q-private]"}},
		{"removed quote", "bo", quoteRef("q-removed"), domain.ReasonSpam, nil, kernel.ErrNotFound,
			[]string{"quotes.ListAny [q-removed]"}},
		{"missing quote", "bo", quoteRef("q-missing"), domain.ReasonSpam, nil, kernel.ErrNotFound,
			[]string{"quotes.ListAny [q-missing]"}},
		{"discussion never answered", "bo", convRef("c-silent"), domain.ReasonSpam, nil, kernel.ErrNotFound,
			[]string{"conversations.ListAny [c-silent]", "quotes.ListAny [q-public]"}},
		{"discussion of a removed quote", "bo", convRef("c-on-removed"), domain.ReasonSpam, nil, kernel.ErrNotFound,
			[]string{"conversations.ListAny [c-on-removed]", "quotes.ListAny [q-removed]"}},
		{"removed discussion", "bo", convRef("c-removed"), domain.ReasonSpam, nil, kernel.ErrNotFound,
			[]string{"conversations.ListAny [c-removed]", "quotes.ListAny [q-public]"}},
		{"missing discussion", "bo", convRef("c-missing"), domain.ReasonSpam, nil, kernel.ErrNotFound,
			[]string{"conversations.ListAny [c-missing]"}},
		{"unknown kind is refused before any lookup", "bo", domain.TargetRef{Kind: "comment", ID: "x"}, domain.ReasonSpam, nil,
			domain.ErrInvalidTarget, nil},
		{"unknown reason is refused before any lookup", "bo", quoteRef("q-public"), "boring", nil, domain.ErrInvalidReason, nil},
		{"already reported", "bo", quoteRef("q-public"), domain.ReasonSpam, domain.ErrAlreadyReported, domain.ErrAlreadyReported,
			[]string{"quotes.ListAny [q-public]", "reports.Create (tx)"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := newWorld()
			reports := &fakeReports{log: w.log, createErr: tt.createErr}
			r, err := NewCreateReport(&portstest.TxManager{}, reports, w.quotes, w.conversations).
				Execute(t.Context(), tt.reporter, tt.ref, tt.reason, " a note ")
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v, want %v", err, tt.wantErr)
			}
			if !reflect.DeepEqual(w.log.calls, tt.wantCalls) {
				t.Errorf("calls = %q, want %q", w.log.calls, tt.wantCalls)
			}
			if err != nil {
				return
			}
			if r.ID == "" || r.ReporterID != tt.reporter || r.OwnerID != "ada" || r.Target != tt.ref ||
				r.Reason != tt.reason || r.Note != "a note" || r.Status != domain.StatusOpen {
				t.Errorf("Execute() = %#v", r)
			}
		})
	}
}

func TestCreateReportStoresTheTargetsOwnID(t *testing.T) {
	w := newWorld()
	// The database matches a uuid however it is spelled.
	w.quotes.byID["Q-PUBLIC"] = w.quotes.byID["q-public"]
	reports := &fakeReports{log: w.log}
	_, err := NewCreateReport(&portstest.TxManager{}, reports, w.quotes, w.conversations).
		Execute(t.Context(), "bo", domain.TargetRef{Kind: domain.TargetQuote, ID: "Q-PUBLIC"}, domain.ReasonSpam, "")
	if err != nil || reports.created[0].Target.ID != "q-public" {
		t.Errorf("stored %#v, %v; want the quote's own id, so reports on it group together", reports.created, err)
	}
}

func TestDismissReports(t *testing.T) {
	ref := domain.TargetRef{Kind: domain.TargetQuote, ID: "q-public"}

	log := &recorder{}
	txm := &portstest.TxManager{}
	n, err := NewDismissReports(txm, &fakeReports{log: log, resolved: 2}).Execute(t.Context(), "root", ref)
	want := []string{"reports.Resolve quote/q-public dismissed by root (tx)"}
	if err != nil || n != 2 || txm.Calls != 1 || !reflect.DeepEqual(log.calls, want) {
		t.Errorf("Execute() = %d, %v, transactions %d, calls %q", n, err, txm.Calls, log.calls)
	}

	if _, err := NewDismissReports(&portstest.TxManager{}, &fakeReports{log: &recorder{}}).Execute(t.Context(), "root", ref); !errors.Is(err, domain.ErrNoOpenReports) {
		t.Errorf("no open reports: err = %v, want ErrNoOpenReports", err)
	}

	log = &recorder{}
	_, err = NewDismissReports(&portstest.TxManager{}, &fakeReports{log: log, resolved: 1}).
		Execute(t.Context(), "root", domain.TargetRef{Kind: domain.TargetQuote})
	if !errors.Is(err, domain.ErrInvalidTarget) || len(log.calls) > 0 {
		t.Errorf("no id: err = %v, calls %q", err, log.calls)
	}
}

func TestListOpenReports(t *testing.T) {
	w := newWorld()
	q := func(id string) domain.TargetRef { return domain.TargetRef{Kind: domain.TargetQuote, ID: id} }
	c := func(id string) domain.TargetRef { return domain.TargetRef{Kind: domain.TargetConversation, ID: id} }
	reports := &fakeReports{log: w.log, open: []domain.Report{
		{ID: "r1", Target: q("q-public"), OwnerID: "ada"},
		{ID: "r2", Target: c("c-answered"), OwnerID: "ada"},
		{ID: "r3", Target: q("q-public"), OwnerID: "ada"},
		{ID: "r4", Target: q("q-deleted"), OwnerID: "ada"},
		{ID: "r5", Target: c("c-on-removed"), OwnerID: "ada"},
	}}
	got, err := NewListOpenReports(reports, w.quotes, w.conversations).Execute(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if reports.listLimit != domain.MaxListed {
		t.Errorf("listed %d reports, want the cap %d", reports.listLimit, domain.MaxListed)
	}
	// Content is read in one batch per context, however many targets there are.
	wantCalls := []string{
		"reports.ListOpen",
		"conversations.ListAny [c-answered c-on-removed]",
		"quotes.ListAny [q-public q-deleted q-removed]",
	}
	if !reflect.DeepEqual(w.log.calls, wantCalls) {
		t.Errorf("calls = %q, want %q", w.log.calls, wantCalls)
	}

	type summary struct {
		ref        domain.TargetRef
		reports    int
		hasContent bool
		quoteID    string
	}
	var sums []summary
	for _, g := range got {
		s := summary{ref: g.Ref, reports: len(g.Reports), hasContent: g.Content != nil}
		if g.Content != nil {
			s.quoteID = g.Content.Quote.ID
		}
		sums = append(sums, s)
	}
	want := []summary{
		{q("q-public"), 2, true, "q-public"},
		{c("c-answered"), 1, true, "q-public"},
		{q("q-deleted"), 1, false, ""},
		{c("c-on-removed"), 1, true, "q-removed"},
	}
	if !reflect.DeepEqual(sums, want) {
		t.Errorf("Execute() = %+v, want %+v", sums, want)
	}

	w = newWorld()
	if got, err := NewListOpenReports(&fakeReports{log: w.log}, w.quotes, w.conversations).Execute(t.Context()); err != nil || len(got) != 0 ||
		!reflect.DeepEqual(w.log.calls, []string{"reports.ListOpen"}) {
		t.Errorf("no reports: %v, %v, calls %q; want no content lookups", got, err, w.log.calls)
	}
}
