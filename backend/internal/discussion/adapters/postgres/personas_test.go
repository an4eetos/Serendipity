package postgres

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"testing"

	catalogpg "github.com/an4eetos/serendipity/backend/internal/catalog/adapters/postgres"
	catalog "github.com/an4eetos/serendipity/backend/internal/catalog/domain"
	"github.com/an4eetos/serendipity/backend/internal/discussion/domain"
	kernelpg "github.com/an4eetos/serendipity/backend/internal/kernel/adapters/postgres"
	"github.com/an4eetos/serendipity/backend/internal/kernel/adapters/postgres/pgtest"
	kernel "github.com/an4eetos/serendipity/backend/internal/kernel/domain"
	quotepg "github.com/an4eetos/serendipity/backend/internal/quote/adapters/postgres"
	quote "github.com/an4eetos/serendipity/backend/internal/quote/domain"
)

const missingID = "00000000-0000-4000-8000-000000000000"

// slugOf matches a new character's slug: base plus 10 random hex digits.
func slugOf(base string) *regexp.Regexp { return regexp.MustCompile(`^` + base + `-[0-9a-f]{10}$`) }

// charactersFixture is a reader with a public and a private quote, and
// another reader.
type charactersFixture struct {
	uid, other   string
	public, priv quote.Quote
	repo         *PersonaRepository
	convs        *ConversationRepository
	quotes       *quotepg.QuoteRepository
	presetID     string
	ctx          context.Context
	db           *kernelpg.DB
	t            *testing.T
}

func newCharactersFixture(t *testing.T, ctx context.Context, db *kernelpg.DB) *charactersFixture {
	t.Helper()
	f := &charactersFixture{uid: pgtest.NewUser(t, ctx, db), other: pgtest.NewUser(t, ctx, db),
		repo: NewPersonaRepository(db), convs: NewConversationRepository(db), quotes: quotepg.NewQuoteRepository(db),
		ctx: ctx, db: db, t: t}
	work, err := catalogpg.NewWorkRepository(db).Ensure(ctx, catalog.Work{Title: "Notes on the Analytical Engine"}, f.uid)
	if err != nil {
		t.Fatal(err)
	}
	q, err := f.quotes.Create(ctx, f.uid, quote.Draft{WorkID: work.ID, Text: "The engine weaves algebraic patterns.",
		Location: "Note A", Reaction: "Lovely.", Source: quote.SourceUpload})
	if err != nil {
		t.Fatal(err)
	}
	if f.public, err = f.quotes.Update(ctx, f.uid, q.ID, quote.Patch{Visibility: ptr(quote.VisibilityPublic)}); err != nil {
		t.Fatal(err)
	}
	if f.priv, err = f.quotes.Create(ctx, f.uid, quote.Draft{WorkID: work.ID, Text: "It has no pretensions to originate anything.",
		Location: "Note G", Reaction: "Hm.", Source: quote.SourceUpload}); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(ctx, `select id from personas where is_preset order by sort_order limit 1`).Scan(&f.presetID); err != nil {
		t.Fatal(err)
	}
	return f
}

func (f *charactersFixture) create(owner, name string) domain.Persona {
	f.t.Helper()
	d, err := domain.CharacterDraft{Name: name, Era: "1840s", ShortBio: "A bio.", Prompt: "You are " + name + "."}.Validate()
	if err != nil {
		f.t.Fatal(err)
	}
	p, err := f.repo.Create(f.ctx, owner, d, domain.SlugBase(name))
	if err != nil {
		f.t.Fatal(err)
	}
	return p
}

// discuss stores an answered discussion of q with p and returns its id.
func (f *charactersFixture) discuss(q quote.Quote, p domain.Persona) string {
	f.t.Helper()
	id, err := f.convs.Create(f.ctx, f.uid, q.ID, p.ID)
	if err != nil {
		f.t.Fatal(err)
	}
	if _, err := f.convs.AppendMessage(f.ctx, id, domain.RoleAssistant, "Do you, though?"); err != nil {
		f.t.Fatal(err)
	}
	return id
}

func (f *charactersFixture) setArchived(id string, archived bool) {
	f.t.Helper()
	p, err := f.repo.GetOwn(f.ctx, f.uid, id)
	if err != nil {
		f.t.Fatal(err)
	}
	if archived {
		p, err = p.Archive(p.UpdatedAt)
	} else {
		p, err = p.Unarchive()
	}
	if err != nil {
		f.t.Fatal(err)
	}
	if _, err := f.repo.Update(f.ctx, p); err != nil {
		f.t.Fatal(err)
	}
}

// attempt runs sql inside a savepoint and returns its error.
func (f *charactersFixture) attempt(sql string, args ...any) error {
	f.t.Helper()
	return pgtest.Attempt(f.t, f.ctx, f.db, func() error {
		_, err := f.db.Exec(f.ctx, sql, args...)
		return err
	})
}

func TestCharacterRepository(t *testing.T) {
	pgtest.Run(t, func(ctx context.Context, db *kernelpg.DB) {
		f := newCharactersFixture(t, ctx, db)
		ada := f.create(f.uid, "Ada Lovelace")
		if !slugOf("ada-lovelace").MatchString(ada.Slug) || ada.IsPreset || ada.OwnerID != f.uid ||
			ada.SystemPrompt != "You are Ada Lovelace." || ada.Status() != domain.PersonaActive || ada.HasConversations {
			t.Errorf("Create() = %#v", ada)
		}
		twin := f.create(f.uid, "Ada Lovelace")
		if twin.Slug == ada.Slug || !slugOf("ada-lovelace").MatchString(twin.Slug) {
			t.Errorf("same name, slugs %q and %q; want distinct", ada.Slug, twin.Slug)
		}
		if long := f.create(f.uid, strings.Repeat("Hypatia ", 10)); len(long.Slug) > 60 {
			t.Errorf("slug %q is longer than 60", long.Slug)
		}
		if odd := f.create(f.uid, "孔子"); !slugOf("character").MatchString(odd.Slug) {
			t.Errorf("slug %q, want character-<hex>", odd.Slug)
		}
		theirs := f.create(f.other, "Grace Hopper")

		t.Run("CountActive and LockOwner", func(t *testing.T) {
			if n, err := f.repo.CountActive(ctx, f.uid); err != nil || n != 4 {
				t.Errorf("CountActive() = %d, %v; want 4", n, err)
			}
			if err := f.repo.LockOwner(ctx, f.uid); err != nil {
				t.Errorf("LockOwner() = %v", err)
			}
			if err := f.repo.LockOwner(ctx, missingID); !errors.Is(err, kernel.ErrNotFound) {
				t.Errorf("LockOwner(missing) = %v, want kernel.ErrNotFound", err)
			}
		})

		t.Run("owner scoping", func(t *testing.T) {
			for name, id := range map[string]string{"another reader's": theirs.ID, "a preset": f.presetID, "missing": missingID} {
				if _, err := f.repo.GetOwn(ctx, f.uid, id); !errors.Is(err, kernel.ErrNotFound) {
					t.Errorf("GetOwn(%s) = %v, want kernel.ErrNotFound", name, err)
				}
			}
			if err := pgtest.Attempt(t, ctx, db, func() error { _, err := f.repo.GetOwn(ctx, f.uid, "not-a-uuid"); return err }); !errors.Is(err, kernel.ErrNotFound) {
				t.Errorf("GetOwn(malformed) = %v, want kernel.ErrNotFound", err)
			}
			if _, err := f.repo.Get(ctx, f.uid, theirs.ID); !errors.Is(err, kernel.ErrNotFound) {
				t.Errorf("Get(another reader's) = %v, want kernel.ErrNotFound", err)
			}
			own, err := f.repo.ListOwn(ctx, f.other)
			if err != nil || len(own) != 1 || own[0].ID != theirs.ID {
				t.Errorf("ListOwn(other) = %#v, %v", own, err)
			}
		})

		f.discuss(f.public, ada)
		f.setArchived(twin.ID, true)

		t.Run("List: presets first, then active own characters", func(t *testing.T) {
			ps, err := f.repo.List(ctx, f.uid)
			if err != nil {
				t.Fatal(err)
			}
			var custom []string
			seenCustom := false
			for _, p := range ps {
				if p.IsPreset && seenCustom {
					t.Errorf("preset %s listed after a custom character", p.Slug)
				}
				if !p.IsPreset {
					seenCustom = true
					custom = append(custom, p.ID)
					if p.OwnerID != f.uid || p.Status() != domain.PersonaActive {
						t.Errorf("listed %#v", p)
					}
				}
			}
			if len(custom) != 3 || contains(custom, twin.ID) || contains(custom, theirs.ID) {
				t.Errorf("custom characters listed %v; want the 3 active ones, without the archived twin", custom)
			}
		})

		t.Run("ListOwn and GetOwn: every state, newest first, with usage", func(t *testing.T) {
			own, err := f.repo.ListOwn(ctx, f.uid)
			if err != nil || len(own) != 4 {
				t.Fatalf("ListOwn() = %d, %v", len(own), err)
			}
			for i := 1; i < len(own); i++ {
				if own[i].CreatedAt.After(own[i-1].CreatedAt) {
					t.Errorf("ListOwn() not newest first at %d", i)
				}
			}
			for _, p := range own {
				if p.ID == ada.ID && !p.HasConversations || p.ID != ada.ID && p.HasConversations {
					t.Errorf("%s HasConversations = %v", p.Name, p.HasConversations)
				}
				if p.ID == twin.ID && p.Status() != domain.PersonaArchived {
					t.Errorf("twin status %s", p.Status())
				}
			}
			got, err := f.repo.GetOwnForUpdate(ctx, f.uid, ada.ID)
			if err != nil || !got.HasConversations || !got.NameLocked() {
				t.Errorf("GetOwnForUpdate() = %#v, %v; want HasConversations", got, err)
			}
			if n, err := f.repo.CountActive(ctx, f.uid); err != nil || n != 3 {
				t.Errorf("CountActive() with one archived = %d, %v; want 3", n, err)
			}
		})

		t.Run("Update: edits, archives and the name lock", func(t *testing.T) {
			p, _ := f.repo.GetOwn(ctx, f.uid, ada.ID)
			p.ShortBio, p.SystemPrompt = "First programmer.", "You are Ada, 1843."
			got, err := f.repo.Update(ctx, p)
			if err != nil || got.ShortBio != "First programmer." || got.SystemPrompt != "You are Ada, 1843." ||
				got.Name != "Ada Lovelace" || !got.HasConversations || got.UpdatedAt.Before(got.CreatedAt) {
				t.Errorf("Update() = %#v, %v", got, err)
			}
			p.Name = "Augusta"
			if err := pgtest.Attempt(t, ctx, db, func() error { _, err := f.repo.Update(ctx, p); return err }); !errors.Is(err, domain.ErrNameLocked) {
				t.Errorf("Update(rename a used character) = %v, want ErrNameLocked", err)
			}
			err = f.attempt(`update personas set name = 'Augusta' where id = $1`, ada.ID)
			if !kernelpg.IsCheckViolationOf(err, "personas_name_locked") {
				t.Errorf("direct rename of a used character = %v, want personas_name_locked", err)
			}
			if err := f.attempt(`update personas set name = name, short_bio = 'same name' where id = $1`, ada.ID); err != nil {
				t.Errorf("rewriting the same name = %v", err)
			}
			unused, _ := f.repo.GetOwn(ctx, f.uid, twin.ID)
			unused.Name = "Augusta King"
			if got, err := f.repo.Update(ctx, unused); err != nil || got.Name != "Augusta King" || got.Status() != domain.PersonaArchived {
				t.Errorf("Update(rename an unused archived character) = %#v, %v", got, err)
			}
			unused.OwnerID = f.other
			if _, err := f.repo.Update(ctx, unused); !errors.Is(err, kernel.ErrNotFound) {
				t.Errorf("Update(as another reader) = %v, want kernel.ErrNotFound", err)
			}
		})

		t.Run("named checks", func(t *testing.T) {
			for _, tt := range []struct{ name, sql, constraint string }{
				{"era of 81", `update personas set era = repeat('é', 81) where id = $1`, "personas_era_length"},
				{"blank prompt", `update personas set system_prompt = '  ' where id = $1`, "personas_prompt_not_blank"},
				{"blank name", `update personas set name = '   ' where id = $1`, "personas_name_not_blank"},
				{"removed without a reason", `update personas set removed_at = now() where id = $1`, "personas_removed_reason"},
				{"a reason without removal", `update personas set removed_reason = 'x' where id = $1`, "personas_removed_reason"},
			} {
				if err := f.attempt(tt.sql, twin.ID); !kernelpg.IsCheckViolationOf(err, tt.constraint) {
					t.Errorf("%s: %v, want %s", tt.name, err, tt.constraint)
				}
			}
			for _, sql := range []string{
				`update personas set archived_at = now() where id = $1`,
				`update personas set removed_at = now(), removed_reason = 'x' where id = $1`,
			} {
				if err := f.attempt(sql, f.presetID); !kernelpg.IsCheckViolationOf(err, "personas_presets_active") {
					t.Errorf("%s on a preset: %v, want personas_presets_active", sql, err)
				}
			}
			if err := f.attempt(`update personas set era = repeat('é', 80) where id = $1`, twin.ID); err != nil {
				t.Errorf("era of 80: %v", err)
			}
		})

		t.Run("Remove and Restore", func(t *testing.T) {
			if err := f.repo.Remove(ctx, f.presetID, "x"); !errors.Is(err, kernel.ErrNotFound) {
				t.Errorf("Remove(preset) = %v", err)
			}
			if err := f.repo.Remove(ctx, missingID, "x"); !errors.Is(err, kernel.ErrNotFound) {
				t.Errorf("Remove(missing) = %v", err)
			}
			if err := f.repo.Restore(ctx, theirs.ID); !errors.Is(err, kernel.ErrNotFound) {
				t.Errorf("Restore(not removed) = %v", err)
			}
			if err := f.repo.Remove(ctx, theirs.ID, "Depicts a living person"); err != nil {
				t.Fatal(err)
			}
			if err := f.repo.Remove(ctx, theirs.ID, "again"); !errors.Is(err, kernel.ErrNotFound) {
				t.Errorf("Remove(removed) = %v", err)
			}
			got, err := f.repo.GetOwn(ctx, f.other, theirs.ID)
			if err != nil || got.Status() != domain.PersonaRemoved || got.RemovedReason != "Depicts a living person" {
				t.Errorf("removed character = %#v, %v", got, err)
			}
			// The owner can't change it any more, and it frees a slot.
			got.ShortBio = "x"
			if _, err := f.repo.Update(ctx, got); !errors.Is(err, kernel.ErrNotFound) {
				t.Errorf("Update(removed) = %v, want kernel.ErrNotFound", err)
			}
			if n, err := f.repo.CountActive(ctx, f.other); err != nil || n != 0 {
				t.Errorf("CountActive() after removal = %d, %v; want 0", n, err)
			}
			if err := f.repo.Restore(ctx, theirs.ID); err != nil {
				t.Fatal(err)
			}
			if got, err := f.repo.GetOwn(ctx, f.other, theirs.ID); err != nil || got.Status() != domain.PersonaActive || got.RemovedReason != "" {
				t.Errorf("restored character = %#v, %v", got, err)
			}
		})
	})
}

func contains(ids []string, id string) bool {
	for _, x := range ids {
		if x == id {
			return true
		}
	}
	return false
}

// R-USABLE, D13: the insert triggers refuse conversations and messages of a
// character that isn't usable, and messages of a removed conversation.
func TestCharacterInsertGuards(t *testing.T) {
	pgtest.Run(t, func(ctx context.Context, db *kernelpg.DB) {
		f := newCharactersFixture(t, ctx, db)
		ada := f.create(f.uid, "Ada")
		conv := f.discuss(f.public, ada)
		if _, err := f.convs.AppendMessage(ctx, conv, domain.RoleUser, "Sometimes."); err != nil {
			t.Fatal(err)
		}

		f.setArchived(ada.ID, true)
		if err := pgtest.Attempt(t, ctx, db, func() error { _, err := f.convs.Create(ctx, f.uid, f.priv.ID, ada.ID); return err }); !errors.Is(err, domain.ErrCharacterArchived) {
			t.Errorf("Create(archived) = %v, want ErrCharacterArchived", err)
		}
		if err := pgtest.Attempt(t, ctx, db, func() error {
			_, err := f.convs.AppendMessage(ctx, conv, domain.RoleAssistant, "Late reply.")
			return err
		}); !errors.Is(err, domain.ErrCharacterArchived) {
			t.Errorf("AppendMessage(archived) = %v, want ErrCharacterArchived", err)
		}
		if err := f.attempt(`insert into messages (conversation_id, role, content) values ($1, 'assistant', 'x')`, conv); !kernelpg.IsCheckViolationOf(err, "personas_archived") {
			t.Errorf("direct insert, archived: %v, want personas_archived", err)
		}

		f.setArchived(ada.ID, false)
		if err := f.repo.Remove(ctx, ada.ID, "spam"); err != nil {
			t.Fatal(err)
		}
		if err := pgtest.Attempt(t, ctx, db, func() error { _, err := f.convs.Create(ctx, f.uid, f.priv.ID, ada.ID); return err }); !errors.Is(err, domain.ErrCharacterRemoved) {
			t.Errorf("Create(removed) = %v, want ErrCharacterRemoved", err)
		}
		if err := pgtest.Attempt(t, ctx, db, func() error {
			_, err := f.convs.AppendMessage(ctx, conv, domain.RoleAssistant, "Late reply.")
			return err
		}); !errors.Is(err, domain.ErrCharacterRemoved) {
			t.Errorf("AppendMessage(removed) = %v, want ErrCharacterRemoved", err)
		}
		if err := f.attempt(`insert into conversations (user_id, quote_id, persona_id) values ($1, $2, $3)`, f.uid, f.priv.ID, ada.ID); !kernelpg.IsCheckViolationOf(err, "personas_removed") {
			t.Errorf("direct insert, removed: %v, want personas_removed", err)
		}
		if err := f.repo.Restore(ctx, ada.ID); err != nil {
			t.Fatal(err)
		}
		if _, err := f.convs.AppendMessage(ctx, conv, domain.RoleAssistant, "Back again."); err != nil {
			t.Errorf("AppendMessage after restore = %v", err)
		}

		// D13: a removed conversation takes no message, whatever its role.
		if err := f.convs.Remove(ctx, conv, "Harassment"); err != nil {
			t.Fatal(err)
		}
		for _, role := range []domain.Role{domain.RoleUser, domain.RoleAssistant} {
			if err := pgtest.Attempt(t, ctx, db, func() error { _, err := f.convs.AppendMessage(ctx, conv, role, "x"); return err }); !errors.Is(err, domain.ErrConversationRemoved) {
				t.Errorf("AppendMessage(%s, removed discussion) = %v, want ErrConversationRemoved", role, err)
			}
		}
		if err := f.attempt(`insert into messages (conversation_id, role, content) values ($1, 'user', 'x')`, conv); !kernelpg.IsCheckViolationOf(err, "conversations_removed") {
			t.Errorf("direct insert, removed discussion: %v, want conversations_removed", err)
		}
		// A removed conversation of a preset is refused as well.
		presetConv := f.discuss(f.priv, domain.Persona{ID: f.presetID})
		if err := f.convs.Remove(ctx, presetConv, "spam"); err != nil {
			t.Fatal(err)
		}
		if err := pgtest.Attempt(t, ctx, db, func() error {
			_, err := f.convs.AppendMessage(ctx, presetConv, domain.RoleUser, "x")
			return err
		}); !errors.Is(err, domain.ErrConversationRemoved) {
			t.Errorf("AppendMessage(removed preset discussion) = %v, want ErrConversationRemoved", err)
		}
		if err := f.convs.Restore(ctx, conv); err != nil {
			t.Fatal(err)
		}
		if _, err := f.convs.AppendMessage(ctx, conv, domain.RoleUser, "Restored."); err != nil {
			t.Errorf("AppendMessage after restoring the discussion = %v", err)
		}
		got, err := f.convs.Get(ctx, f.uid, conv)
		if err != nil || len(got.Messages) != 4 {
			t.Errorf("Get() = %d messages, %v; want the 4 accepted ones", len(got.Messages), err)
		}
	})
}

// P1: public pages leave out the discussions of a removed character, and
// show them again once it is restored. The owner and moderators still see
// them.
func TestRemovedCharacterLeavesPublicReads(t *testing.T) {
	pgtest.Run(t, func(ctx context.Context, db *kernelpg.DB) {
		f := newCharactersFixture(t, ctx, db)
		ada := f.create(f.uid, "Ada")
		withAda := f.discuss(f.public, ada)
		withPreset := f.discuss(f.public, domain.Persona{ID: f.presetID})
		archivedConv := f.discuss(f.public, f.create(f.uid, "Bo"))
		bo, err := f.convs.Get(ctx, f.uid, archivedConv)
		if err != nil {
			t.Fatal(err)
		}
		f.setArchived(bo.Persona.ID, true)

		ids := func(cs []domain.Conversation) []string {
			out := []string{}
			for _, c := range cs {
				out = append(out, c.ID)
			}
			return out
		}
		public, err := f.convs.ListForQuotes(ctx, f.uid, []string{f.public.ID})
		if err != nil || len(public) != 3 {
			t.Fatalf("ListForQuotes() before = %v, %v; want all 3 (archiving hides nothing)", ids(public), err)
		}

		if err := f.repo.Remove(ctx, ada.ID, "spam"); err != nil {
			t.Fatal(err)
		}
		public, err = f.convs.ListForQuotes(ctx, f.uid, []string{f.public.ID})
		if got := ids(public); err != nil || len(got) != 2 || contains(got, withAda) || !contains(got, withPreset) || !contains(got, archivedConv) {
			t.Errorf("ListForQuotes() after removing the character = %v, %v; want it without %s", got, err, withAda)
		}
		own, err := f.convs.ListForQuote(ctx, f.uid, f.public.ID)
		if got := ids(own); err != nil || !contains(got, withAda) {
			t.Errorf("owner's ListForQuote() = %v, %v; want the discussion kept", got, err)
		}
		for _, c := range own {
			if c.ID == withAda && c.Persona.Status() != domain.PersonaRemoved {
				t.Errorf("owner's view of the character: %s, want removed", c.Persona.Status())
			}
		}
		if anyOwner, err := f.convs.ListAny(ctx, []string{withAda}); err != nil || len(anyOwner) != 1 {
			t.Errorf("moderators' ListAny() = %v, %v", ids(anyOwner), err)
		}

		if err := f.repo.Restore(ctx, ada.ID); err != nil {
			t.Fatal(err)
		}
		if public, err := f.convs.ListForQuotes(ctx, f.uid, []string{f.public.ID}); err != nil || !contains(ids(public), withAda) {
			t.Errorf("ListForQuotes() after restoring = %v, %v", ids(public), err)
		}
	})
}

// P4: ListAny reports whether a public page shows a discussion with the
// character, and leaves presets and missing ids out.
func TestPersonaModerationListAny(t *testing.T) {
	pgtest.Run(t, func(ctx context.Context, db *kernelpg.DB) {
		f := newCharactersFixture(t, ctx, db)
		answered := f.create(f.uid, "Answered")
		f.discuss(f.public, answered)
		onPrivate := f.create(f.uid, "On Private")
		f.discuss(f.priv, onPrivate)
		unanswered := f.create(f.uid, "Unanswered")
		if _, err := f.convs.Create(ctx, f.uid, f.public.ID, unanswered.ID); err != nil {
			t.Fatal(err)
		}
		inRemovedConv := f.create(f.uid, "In Removed Discussion")
		if err := f.convs.Remove(ctx, f.discuss(f.public, inRemovedConv), "spam"); err != nil {
			t.Fatal(err)
		}
		unused := f.create(f.uid, "Unused")

		ms, err := f.repo.ListAny(ctx, []string{answered.ID, onPrivate.ID, unanswered.ID, inRemovedConv.ID, unused.ID, f.presetID, missingID})
		if err != nil {
			t.Fatal(err)
		}
		got := map[string]bool{}
		for _, m := range ms {
			got[m.Persona.Name] = m.InPublicDiscussion
			if m.Persona.SystemPrompt == "" || m.Persona.OwnerID != f.uid {
				t.Errorf("ListAny() persona %#v; want the prompt and the owner", m.Persona)
			}
		}
		want := map[string]bool{"Answered": true, "On Private": false, "Unanswered": false, "In Removed Discussion": false, "Unused": false}
		if len(got) != len(want) {
			t.Errorf("ListAny() = %v; want %v (no preset, no missing id)", got, want)
		}
		for name, w := range want {
			if v, ok := got[name]; !ok || v != w {
				t.Errorf("%s: InPublicDiscussion = %v (listed %v), want %v", name, v, ok, w)
			}
		}

		// A removed quote takes its discussions off public pages too.
		if err := f.quotes.Remove(ctx, f.public.ID, "copyright"); err != nil {
			t.Fatal(err)
		}
		if ms, err := f.repo.ListAny(ctx, []string{answered.ID}); err != nil || len(ms) != 1 || ms[0].InPublicDiscussion {
			t.Errorf("ListAny() after removing the quote = %#v, %v; want not in a public discussion", ms, err)
		}
		if none, err := f.repo.ListAny(ctx, nil); err != nil || none == nil || len(none) != 0 {
			t.Errorf("ListAny(nil) = %#v, %v; want empty", none, err)
		}
	})
}
