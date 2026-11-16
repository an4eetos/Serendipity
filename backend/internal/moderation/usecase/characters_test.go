package usecase

import (
	"errors"
	"reflect"
	"testing"

	kernel "github.com/an4eetos/serendipity/backend/internal/kernel/domain"
	"github.com/an4eetos/serendipity/backend/internal/kernel/ports/portstest"
	"github.com/an4eetos/serendipity/backend/internal/moderation/domain"
	quote "github.com/an4eetos/serendipity/backend/internal/quote/domain"
)

func charRef(id string) domain.TargetRef {
	return domain.TargetRef{Kind: domain.TargetCharacter, ID: id}
}

func TestCreateReportOnCharacters(t *testing.T) {
	convRef := func(id string) domain.TargetRef { return domain.TargetRef{Kind: domain.TargetConversation, ID: id} }
	tests := []struct {
		name      string
		reporter  string
		ref       domain.TargetRef
		wantErr   error
		wantCalls []string
	}{
		{"someone else's character in a public discussion", "bo", charRef("ch-public"), nil,
			[]string{"personas.ListAny [ch-public]", "profiles.IsPublic ada", "reports.Create (tx)"}},
		{"an archived character still in a public discussion", "bo", charRef("ch-archived"), nil,
			[]string{"personas.ListAny [ch-archived]", "profiles.IsPublic ada", "reports.Create (tx)"}},
		{"a character in no public discussion", "bo", charRef("ch-unseen"), kernel.ErrNotFound,
			[]string{"personas.ListAny [ch-unseen]"}},
		{"a removed character", "bo", charRef("ch-removed"), kernel.ErrNotFound,
			[]string{"personas.ListAny [ch-removed]"}},
		{"a character on a private profile", "bo", charRef("ch-on-private-profile"), kernel.ErrNotFound,
			[]string{"personas.ListAny [ch-on-private-profile]", "profiles.IsPublic cy"}},
		{"own character", "ada", charRef("ch-public"), domain.ErrOwnContent,
			[]string{"personas.ListAny [ch-public]", "profiles.IsPublic ada"}},
		{"a missing character or a preset", "bo", charRef("ch-missing"), kernel.ErrNotFound,
			[]string{"personas.ListAny [ch-missing]"}},
		// P3: a discussion whose character is removed is on no public page.
		{"a discussion whose character is removed", "bo", convRef("c-removed-character"), kernel.ErrNotFound,
			[]string{"conversations.ListAny [c-removed-character]", "quotes.ListAny [q-public]"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := newWorld()
			reports := &fakeReports{log: w.log}
			r, err := NewCreateReport(&portstest.TxManager{}, reports, w.profiles, w.quotes, w.conversations, w.personas).
				Execute(t.Context(), tt.reporter, tt.ref, domain.ReasonLivingPerson, "")
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v, want %v", err, tt.wantErr)
			}
			if !reflect.DeepEqual(w.log.calls, tt.wantCalls) {
				t.Errorf("calls = %q, want %q", w.log.calls, tt.wantCalls)
			}
			if err != nil {
				if len(reports.created) != 0 {
					t.Errorf("stored %#v after a refusal", reports.created)
				}
				return
			}
			if r.Target != tt.ref || r.OwnerID != "ada" || r.Reason != domain.ReasonLivingPerson || r.Status != domain.StatusOpen {
				t.Errorf("Execute() = %#v", r)
			}
		})
	}
}

func TestHideCharacter(t *testing.T) {
	t.Run("removes it and closes its reports in one transaction", func(t *testing.T) {
		w := newWorld()
		txm := &portstest.TxManager{}
		td, err := NewHideTarget(txm, &fakeReports{log: w.log}, &fakeTakedowns{log: w.log}, w.quotes, w.conversations, w.personas).
			Execute(t.Context(), "root", charRef("ch-public"), "  Depicts a living person  ")
		if err != nil {
			t.Fatal(err)
		}
		want := []string{
			"personas.ListAny [ch-public]",
			`personas.Remove ch-public "Depicts a living person" (tx)`,
			`takedowns.Create character/ch-public "Depicts a living person" (tx)`,
			"reports.Resolve character/ch-public actioned by root (tx)",
		}
		if !reflect.DeepEqual(w.log.calls, want) {
			t.Errorf("calls = %q, want %q", w.log.calls, want)
		}
		if txm.Calls != 1 || td.Target != charRef("ch-public") || td.OwnerID != "ada" {
			t.Errorf("Execute() = %#v, transactions %d", td, txm.Calls)
		}
		if p := w.personas.byID["ch-public"].Persona; !p.Removed() || p.RemovedReason != "Depicts a living person" {
			t.Errorf("character after hiding = %#v", p)
		}
	})
	t.Run("an archived character can be hidden", func(t *testing.T) {
		w := newWorld()
		_, err := NewHideTarget(&portstest.TxManager{}, &fakeReports{log: w.log}, &fakeTakedowns{log: w.log}, w.quotes, w.conversations, w.personas).
			Execute(t.Context(), "root", charRef("ch-archived"), "x")
		if err != nil || !w.personas.byID["ch-archived"].Persona.Removed() {
			t.Errorf("err = %v, character %#v", err, w.personas.byID["ch-archived"].Persona)
		}
	})
	t.Run("already hidden", func(t *testing.T) {
		w := newWorld()
		_, err := NewHideTarget(&portstest.TxManager{}, &fakeReports{log: w.log}, &fakeTakedowns{log: w.log}, w.quotes, w.conversations, w.personas).
			Execute(t.Context(), "root", charRef("ch-removed"), "again")
		if !errors.Is(err, domain.ErrAlreadyRemoved) || !reflect.DeepEqual(w.log.calls, []string{"personas.ListAny [ch-removed]"}) {
			t.Errorf("err = %v, calls %q", err, w.log.calls)
		}
	})
	t.Run("missing", func(t *testing.T) {
		w := newWorld()
		_, err := NewHideTarget(&portstest.TxManager{}, &fakeReports{log: w.log}, &fakeTakedowns{log: w.log}, w.quotes, w.conversations, w.personas).
			Execute(t.Context(), "root", charRef("ch-missing"), "x")
		if !errors.Is(err, kernel.ErrNotFound) {
			t.Errorf("err = %v, want kernel.ErrNotFound", err)
		}
	})
}

func TestRestoreCharacter(t *testing.T) {
	const wordCap quote.CoverageCap = 777
	newTakedowns := func(w world) *fakeTakedowns {
		return &fakeTakedowns{log: w.log, byID: map[string]domain.Takedown{
			"td-char":     {ID: "td-char", Target: charRef("ch-removed"), OwnerID: "ada"},
			"td-char-del": {ID: "td-char-del", Target: charRef("ch-deleted"), OwnerID: "ada"},
		}}
	}
	tests := []struct {
		name      string
		takedown  string
		wantCalls []string
	}{
		// D7: a restored character ignores the coverage cap and its owner's
		// cap of active characters.
		{"a character, without the coverage cap", "td-char", []string{
			"takedowns.Get td-char",
			"personas.ListAny [ch-removed]",
			"personas.Restore ch-removed (tx)",
			"takedowns.MarkRestored td-char by root (tx)",
			"reports.Resolve character/ch-removed dismissed by root (tx)",
		}},
		{"a character the owner deleted", "td-char-del", []string{
			"takedowns.Get td-char-del",
			"personas.ListAny [ch-deleted]",
			"takedowns.MarkRestored td-char-del by root (tx)",
			"reports.Resolve character/ch-deleted dismissed by root (tx)",
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := newWorld()
			txm := &portstest.TxManager{}
			td, err := NewRestoreTarget(txm, &fakeReports{log: w.log}, newTakedowns(w), w.quotes, w.conversations, w.personas, wordCap).
				Execute(t.Context(), "root", tt.takedown)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(w.log.calls, tt.wantCalls) {
				t.Errorf("calls = %q, want %q", w.log.calls, tt.wantCalls)
			}
			if txm.Calls != 1 || td.RestoredAt == nil {
				t.Errorf("Execute() = %#v, transactions %d", td, txm.Calls)
			}
		})
	}
	t.Run("the restored character is usable again", func(t *testing.T) {
		w := newWorld()
		_, err := NewRestoreTarget(&portstest.TxManager{}, &fakeReports{log: w.log}, newTakedowns(w), w.quotes, w.conversations, w.personas, wordCap).
			Execute(t.Context(), "root", "td-char")
		if p := w.personas.byID["ch-removed"].Persona; err != nil || p.Removed() || p.RemovedReason != "" || p.CheckUsable() != nil {
			t.Errorf("character after restore = %#v, %v", p, err)
		}
	})
}

func TestListOpenReportsOnCharacters(t *testing.T) {
	w := newWorld()
	reports := &fakeReports{log: w.log, open: []domain.Report{
		{ID: "r1", Target: charRef("ch-public"), OwnerID: "ada", Reason: domain.ReasonLivingPerson},
		{ID: "r2", Target: domain.TargetRef{Kind: domain.TargetQuote, ID: "q-public"}, OwnerID: "ada"},
		{ID: "r3", Target: charRef("ch-archived"), OwnerID: "ada"},
		{ID: "r4", Target: charRef("ch-public"), OwnerID: "ada"},
		{ID: "r5", Target: charRef("ch-deleted"), OwnerID: "ada"},
	}}
	got, err := NewListOpenReports(reports, w.quotes, w.conversations, w.personas).Execute(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	// One PersonaModeration.ListAny for every character, without repeats.
	wantCalls := []string{
		"reports.ListOpen",
		"personas.ListAny [ch-public ch-archived ch-deleted]",
		"quotes.ListAny [q-public]",
	}
	if !reflect.DeepEqual(w.log.calls, wantCalls) {
		t.Errorf("calls = %q, want %q", w.log.calls, wantCalls)
	}
	byRef := map[domain.TargetRef]domain.ReportedTarget{}
	for _, g := range got {
		byRef[g.Ref] = g
	}
	pub := byRef[charRef("ch-public")]
	if len(pub.Reports) != 2 || pub.Content == nil || pub.Content.Character == nil ||
		pub.Content.Character.SystemPrompt != "Prompt of ch-public" || pub.Content.Quote != nil || pub.Content.Conversation != nil {
		t.Errorf("ch-public = %+v; want 2 reports and the character, prompt included, without quote or discussion", pub)
	}
	if c := byRef[charRef("ch-archived")].Content; c == nil || c.Character == nil || c.Character.Status() != "archived" {
		t.Errorf("ch-archived content = %+v", c)
	}
	if c := byRef[charRef("ch-deleted")].Content; c != nil {
		t.Errorf("ch-deleted content = %+v, want none", c)
	}
	if c := byRef[domain.TargetRef{Kind: domain.TargetQuote, ID: "q-public"}].Content; c == nil || c.Character != nil {
		t.Errorf("q-public content = %+v, want the quote and no character", c)
	}
}

func TestListTakedownsOfCharacters(t *testing.T) {
	w := newWorld()
	takedowns := &fakeTakedowns{log: w.log, active: []domain.Takedown{
		{ID: "td1", Target: charRef("ch-removed")},
		{ID: "td2", Target: charRef("ch-deleted")},
	}}
	views, err := NewListTakedowns(takedowns, w.quotes, w.conversations, w.personas).Execute(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	wantCalls := []string{"takedowns.ListActive", "personas.ListAny [ch-removed ch-deleted]"}
	if !reflect.DeepEqual(w.log.calls, wantCalls) {
		t.Errorf("calls = %q, want %q", w.log.calls, wantCalls)
	}
	if len(views) != 2 || views[0].Content == nil || !views[0].Content.Removed || views[0].Content.Character == nil ||
		views[0].Content.Character.RemovedReason != "living person" || views[1].Content != nil {
		t.Errorf("Execute() = %+v; want the removed character for td1, nothing for td2", views)
	}
}
