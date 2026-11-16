//go:build e2e

package e2e

import (
	"net/http"
	"regexp"
	"strings"
	"testing"
)

const (
	limitMsg            = "you've reached your limit of active characters — archive one first"
	nameLockedMsg       = "the name can't change once the character has been in a discussion"
	archivedMsg         = "this character is archived — unarchive it to continue"
	characterRemovedMsg = "a moderator removed this character"
	discussionRemoved   = "a moderator removed this discussion — it can't continue"
	characterTakedown   = "e2e: depicts a living person"
	discussionTakedown  = "e2e: harassment in the discussion"
)

var characterSlug = regexp.MustCompile(`^e2e-ada-lovelace-[0-9a-f]{10}$`)

func characterReq(name, prompt string) map[string]string {
	return map[string]string{"name": name, "era": "1840s", "short_bio": "e2e: first programmer.", "prompt": prompt}
}

// characters reads A's own characters.
func (s *scenario) characters(t *testing.T) characterListBody {
	t.Helper()
	return expectDecode[characterListBody](t, s.api.call(t, http.MethodGet, "/characters", s.a.token, nil), http.StatusOK)
}

// noPrompt stops the check when r's body carries A's prompt.
func (s *scenario) noPrompt(t *testing.T, r response) {
	t.Helper()
	need(t, s.charPrompt)
	if strings.Contains(string(r.body), s.charPrompt) || strings.Contains(string(r.body), `"prompt"`) ||
		strings.Contains(string(r.body), `"system_prompt"`) {
		t.Fatalf("the response carries a prompt: %s", r)
	}
}

// pickerHas reports whether reader's picker (GET /personas) lists id.
func (s *scenario) pickerHas(t *testing.T, token, id string) bool {
	t.Helper()
	r := s.api.call(t, http.MethodGet, "/personas", token, nil)
	s.noPrompt(t, r)
	for _, p := range expectDecode[[]personaBody](t, r, http.StatusOK) {
		if p.ID == id {
			return true
		}
	}
	return false
}

// stepCharacters: A creates characters up to the cap, edits one, which is
// name-locked once it has been in a discussion; its prompt reaches only A.
func (s *scenario) stepCharacters(t *testing.T) {
	a := s.a
	s.charPrompt = "e2e secret prompt " + s.tag + ": you are Ada Lovelace, writing in 1843."
	t.Run("A has no characters and a cap of 2", func(t *testing.T) {
		l := s.characters(t)
		if l.Characters == nil || len(l.Characters) != 0 || l.Active != 0 || l.ActiveLimit != 2 {
			t.Fatalf("GET /characters = %+v, want [] with active_limit 2", l)
		}
	})
	t.Run("the admin who hasn't accepted the terms can't create one", func(t *testing.T) {
		r := s.api.call(t, http.MethodPost, "/characters", s.c.token, characterReq("e2e Ada", "x"))
		expectError(t, r, http.StatusForbidden, gatedMsg)
	})
	t.Run("a blank prompt is 400", func(t *testing.T) {
		r := s.api.call(t, http.MethodPost, "/characters", a.token, characterReq("e2e Ada", "   "))
		expectError(t, r, http.StatusBadRequest, "prompt must be 1–8000 characters")
	})
	t.Run("creating a character is 201", func(t *testing.T) {
		c := expectDecode[characterBody](t, s.api.call(t, http.MethodPost, "/characters", a.token,
			characterReq("  e2e Ada Lovelace  ", "  "+s.charPrompt+"  ")), http.StatusCreated)
		s.char1 = c.ID
		if c.Name != "e2e Ada Lovelace" || c.Prompt != s.charPrompt || c.Status != "active" || c.NameLocked ||
			!characterSlug.MatchString(c.Slug) || c.ArchivedAt != nil {
			t.Fatalf("created %+v, want the trimmed fields, status active, unlocked, slug e2e-ada-lovelace-<hex>", c)
		}
	})
	t.Run("a second character is 201", func(t *testing.T) {
		c := expectDecode[characterBody](t, s.api.call(t, http.MethodPost, "/characters", a.token,
			characterReq("e2e Charles Babbage", "e2e: you are Charles Babbage.")), http.StatusCreated)
		s.char2 = c.ID
	})
	t.Run("a third is 409 at the cap of 2", func(t *testing.T) {
		r := s.api.call(t, http.MethodPost, "/characters", a.token, characterReq("e2e Mary Somerville", "x"))
		expectError(t, r, http.StatusConflict, limitMsg)
	})
	t.Run("A's picker lists both after the presets, without prompts", func(t *testing.T) {
		need(t, s.char1, s.char2)
		if !s.pickerHas(t, a.token, s.char1) || !s.pickerHas(t, a.token, s.char2) {
			t.Fatal("A's picker lacks a character")
		}
	})
	t.Run("B's picker doesn't list them, and B can't edit them", func(t *testing.T) {
		need(t, s.char1)
		if s.pickerHas(t, s.b.token, s.char1) {
			t.Error("B's picker lists A's character")
		}
		r := s.api.call(t, http.MethodPatch, "/characters/"+s.char1, s.b.token, map[string]string{"short_bio": "x"})
		expectError(t, r, http.StatusNotFound, "not found")
	})
	t.Run("an unused character can be renamed", func(t *testing.T) {
		need(t, s.char1)
		c := expectDecode[characterBody](t, s.api.call(t, http.MethodPatch, "/characters/"+s.char1, a.token,
			map[string]string{"name": "e2e Ada King"}), http.StatusOK)
		if c.Name != "e2e Ada King" || c.Prompt != s.charPrompt {
			t.Fatalf("renamed %+v", c)
		}
	})
	t.Run("an answered discussion of q1 with it is stored with SQL", func(t *testing.T) {
		need(t, s.q1, s.char1)
		id, err := insertAnsweredConversationWith(t.Context(), s.db, a.id, s.q1, s.char1)
		if err != nil {
			t.Fatal(err)
		}
		s.charConv = id
	})
	t.Run("it is name-locked now: renaming is 409, the bio can change", func(t *testing.T) {
		need(t, s.char1, s.charConv)
		if c := s.characters(t).character(s.char1); c == nil || !c.NameLocked {
			t.Fatalf("listed %+v, want name_locked", c)
		}
		r := s.api.call(t, http.MethodPatch, "/characters/"+s.char1, a.token, map[string]string{"name": "e2e Augusta"})
		expectError(t, r, http.StatusConflict, nameLockedMsg)
		c := expectDecode[characterBody](t, s.api.call(t, http.MethodPatch, "/characters/"+s.char1, a.token,
			map[string]string{"short_bio": "e2e: Countess of Lovelace."}), http.StatusOK)
		if c.Name != "e2e Ada King" || c.ShortBio != "e2e: Countess of Lovelace." {
			t.Fatalf("edited %+v", c)
		}
	})
	s.checkPromptPrivacy(t)
}

// checkPromptPrivacy (R5): the public profile shows the discussion with the
// character's name, era and bio; no reader response carries the prompt.
func (s *scenario) checkPromptPrivacy(t *testing.T) {
	t.Run("the public profile shows the discussion, without the prompt", func(t *testing.T) {
		need(t, s.charConv)
		for _, token := range []string{"", s.a.token, s.b.token} {
			r := s.api.call(t, http.MethodGet, "/profiles/"+s.a.username, token, nil)
			s.noPrompt(t, r)
			p := expectDecode[publicProfileBody](t, r, http.StatusOK)
			c := p.conversation(s.charConv)
			if c == nil || c.Persona.Name != "e2e Ada King" || c.Persona.Era != "1840s" ||
				c.Persona.ShortBio != "e2e: Countess of Lovelace." || c.Persona.IsPreset {
				t.Fatalf("discussion %+v, want it with the character's name, era and bio", c)
			}
		}
	})
	t.Run("the owner's quote page carries no prompt", func(t *testing.T) {
		need(t, s.q1)
		s.noPrompt(t, s.api.call(t, http.MethodGet, "/quotes/"+s.q1, s.a.token, nil))
	})
	t.Run("only the owner's GET characters carries it", func(t *testing.T) {
		need(t, s.char1)
		if c := s.characters(t).character(s.char1); c == nil || c.Prompt != s.charPrompt {
			t.Fatalf("A's character %+v, want the prompt", c)
		}
		if l := expectDecode[characterListBody](t, s.api.call(t, http.MethodGet, "/characters", s.b.token, nil),
			http.StatusOK); len(l.Characters) != 0 {
			t.Fatalf("B lists %+v, want none", l.Characters)
		}
	})
}

// stepCharacterArchive: an archived character leaves the picker, can't
// start or continue a discussion, and frees a slot; unarchiving needs one.
func (s *scenario) stepCharacterArchive(t *testing.T) {
	a := s.a
	t.Run("archiving is 200, and again is 200", func(t *testing.T) {
		need(t, s.char1)
		for range 2 {
			c := expectDecode[characterBody](t, s.api.call(t, http.MethodPost, "/characters/"+s.char1+"/archive",
				a.token, nil), http.StatusOK)
			if c.Status != "archived" || c.ArchivedAt == nil {
				t.Fatalf("archived %+v", c)
			}
		}
	})
	t.Run("the picker hides it", func(t *testing.T) {
		need(t, s.char1)
		if s.pickerHas(t, a.token, s.char1) {
			t.Fatal("A's picker lists the archived character")
		}
	})
	t.Run("starting a discussion with it is 409", func(t *testing.T) {
		need(t, s.q1, s.char1)
		r := s.api.call(t, http.MethodPost, "/conversations", a.token, map[string]string{"quote_id": s.q1, "persona_id": s.char1})
		expectError(t, r, http.StatusConflict, archivedMsg)
	})
	t.Run("its thread is read-only: posting is 409 and stores nothing", func(t *testing.T) {
		need(t, s.charConv)
		s.refusedMessage(t, archivedMsg)
	})
	t.Run("its public discussion stays", func(t *testing.T) {
		need(t, s.charConv)
		if !s.publicProfile(t).hasConversation(s.charConv) {
			t.Fatal("archiving took the discussion off the profile")
		}
	})
	t.Run("the slot it freed takes a new character", func(t *testing.T) {
		c := expectDecode[characterBody](t, s.api.call(t, http.MethodPost, "/characters", a.token,
			characterReq("e2e Mary Somerville", "e2e: you are Mary Somerville.")), http.StatusCreated)
		s.char3 = c.ID
	})
	t.Run("unarchiving at the cap is 409", func(t *testing.T) {
		need(t, s.char1, s.char3)
		r := s.api.call(t, http.MethodPost, "/characters/"+s.char1+"/unarchive", a.token, nil)
		expectError(t, r, http.StatusConflict, limitMsg)
	})
	t.Run("after archiving another, unarchiving is 200", func(t *testing.T) {
		need(t, s.char1, s.char3)
		expect(t, s.api.call(t, http.MethodPost, "/characters/"+s.char3+"/archive", a.token, nil), http.StatusOK)
		c := expectDecode[characterBody](t, s.api.call(t, http.MethodPost, "/characters/"+s.char1+"/unarchive",
			a.token, nil), http.StatusOK)
		if c.Status != "active" || c.ArchivedAt != nil {
			t.Fatalf("unarchived %+v", c)
		}
		if l := s.characters(t); l.Active != 2 || len(l.Characters) != 3 {
			t.Errorf("GET /characters: %d active of %d, want 2 of 3", l.Active, len(l.Characters))
		}
		if !s.pickerHas(t, a.token, s.char1) {
			t.Error("the picker doesn't list the unarchived character")
		}
	})
}

// refusedMessage posts in the character's discussion and checks that it is
// refused with msg before anything is stored or streamed.
func (s *scenario) refusedMessage(t *testing.T, msg string) {
	t.Helper()
	before, err := messageCount(t.Context(), s.db, s.charConv)
	if err != nil {
		t.Fatal(err)
	}
	r := s.api.call(t, http.MethodPost, "/conversations/"+s.charConv+"/messages", s.a.token,
		map[string]string{"content": "e2e: and yet?"})
	expectError(t, r, http.StatusConflict, msg)
	if after, err := messageCount(t.Context(), s.db, s.charConv); err != nil || after != before {
		t.Fatalf("messages %d → %d (%v); want none stored", before, after, err)
	}
}

// stepCharacterModeration: B reports the character; the admin sees its
// prompt and hides it, which takes its discussions off public pages until it
// is restored. A hidden discussion (D13) takes no new message.
func (s *scenario) stepCharacterModeration(t *testing.T) {
	b := s.b
	t.Run("B reports the character as a living person: 201, without the prompt", func(t *testing.T) {
		need(t, s.char1)
		r := s.api.call(t, http.MethodPost, "/reports", b.token, map[string]string{
			"target_kind": "character", "target_id": s.char1, "reason": "living_person", "note": "e2e"})
		s.noPrompt(t, r)
		rep := expectDecode[reportBody](t, r, http.StatusCreated)
		if rep.TargetKind != "character" || rep.TargetID != s.char1 || rep.Status != "open" {
			t.Fatalf("report %+v", rep)
		}
	})
	t.Run("A can't report its own character", func(t *testing.T) {
		need(t, s.char1)
		r := s.api.call(t, http.MethodPost, "/reports", s.a.token, reportReq("character", s.char1))
		expectError(t, r, http.StatusBadRequest, "you can't report your own content")
	})
	t.Run("a character in no public discussion can't be reported", func(t *testing.T) {
		need(t, s.char2)
		r := s.api.call(t, http.MethodPost, "/reports", b.token, reportReq("character", s.char2))
		expectError(t, r, http.StatusNotFound, "not found")
	})
	t.Run("the admin sees the report with the prompt", func(t *testing.T) {
		need(t, s.char1)
		got := s.adminContent(t, "/admin/reports", s.char1)
		if got == nil || got.TargetKind != "character" || got.Character == nil || got.Character.Prompt != s.charPrompt ||
			len(got.Reports) != 1 || got.Reports[0].Reason != "living_person" || got.Reports[0].ReporterID != b.id {
			t.Fatalf("reported character %+v, want B's living_person report with the prompt", got)
		}
	})
	var before *publicBookBody
	t.Run("hiding the character is 201", func(t *testing.T) {
		need(t, s.char1, s.workA)
		before = s.publicProfile(t).book(s.workA)
		td := expectDecode[takedownBody](t, s.api.call(t, http.MethodPost, "/admin/takedowns", s.c.token,
			map[string]string{"target_kind": "character", "target_id": s.char1, "reason": characterTakedown}), http.StatusCreated)
		s.charTakedown = td.ID
		if td.TargetKind != "character" || td.OwnerID != s.a.id {
			t.Fatalf("takedown %+v", td)
		}
	})
	t.Run("its discussion leaves the profile; the quote and the counts stay", func(t *testing.T) {
		need(t, s.charTakedown, s.q1)
		p := s.publicProfile(t)
		if p.hasConversation(s.charConv) || p.quote(s.q1) == nil || !p.hasConversation(s.conv) {
			t.Fatalf("q1 %+v; want it listed with the preset's discussion only", p.quote(s.q1))
		}
		after := p.book(s.workA)
		if before == nil || after == nil || after.PublicQuotes != before.PublicQuotes || len(after.Trail.Points) != len(before.Trail.Points) {
			t.Errorf("shared book before %+v, after %+v; want the counts and trail unchanged", before, after)
		}
	})
	s.checkHiddenCharacter(t)
	s.checkRestoredCharacter(t)
	s.checkRemovedDiscussion(t)
}

// checkHiddenCharacter: what the hidden character looks like to readers,
// its owner and admins.
func (s *scenario) checkHiddenCharacter(t *testing.T) {
	a := s.a
	t.Run("reporting its discussion or it again is 404", func(t *testing.T) {
		need(t, s.charTakedown)
		expectError(t, s.api.call(t, http.MethodPost, "/reports", s.b.token, reportReq("conversation", s.charConv)),
			http.StatusNotFound, "not found")
		expectError(t, s.api.call(t, http.MethodPost, "/reports", s.b.token, reportReq("character", s.char1)),
			http.StatusNotFound, "not found")
	})
	t.Run("the owner sees the reason and can't edit, archive or unarchive it", func(t *testing.T) {
		need(t, s.charTakedown)
		c := s.characters(t).character(s.char1)
		if c == nil || c.Status != "removed" || c.RemovedReason != characterTakedown {
			t.Fatalf("A's character %+v, want removed with the reason", c)
		}
		expectError(t, s.api.call(t, http.MethodPatch, "/characters/"+s.char1, a.token, map[string]string{"short_bio": "x"}),
			http.StatusConflict, characterRemovedMsg)
		for _, action := range []string{"archive", "unarchive"} {
			expectError(t, s.api.call(t, http.MethodPost, "/characters/"+s.char1+"/"+action, a.token, nil),
				http.StatusConflict, characterRemovedMsg)
		}
	})
	t.Run("the owner's thread shows it removed, and posting is 409", func(t *testing.T) {
		need(t, s.charTakedown)
		q := expectDecode[quoteBody](t, s.api.call(t, http.MethodGet, "/quotes/"+s.q1, a.token, nil), http.StatusOK)
		var conv *conversationBody
		for i := range q.Conversations {
			if q.Conversations[i].ID == s.charConv {
				conv = &q.Conversations[i]
			}
		}
		if conv == nil || conv.Persona.Status != "removed" {
			t.Fatalf("thread %+v, want the discussion with persona status removed", conv)
		}
		s.refusedMessage(t, characterRemovedMsg)
	})
	t.Run("the picker omits it, and its slot is free", func(t *testing.T) {
		need(t, s.charTakedown)
		if s.pickerHas(t, a.token, s.char1) {
			t.Error("the picker lists the removed character")
		}
		if l := s.characters(t); l.Active != 1 {
			t.Errorf("active %d, want 1: a removed character frees its slot", l.Active)
		}
	})
	t.Run("the admin's takedown list shows it with the prompt", func(t *testing.T) {
		need(t, s.charTakedown)
		got := s.adminContent(t, "/admin/takedowns", s.char1)
		if got == nil || got.ID != s.charTakedown || got.Character == nil || got.Character.Prompt != s.charPrompt ||
			got.Character.Status != "removed" {
			t.Fatalf("takedown %+v, want the removed character with its prompt", got)
		}
	})
}

// checkRestoredCharacter: restoring brings the discussion back and closes
// the character's report.
func (s *scenario) checkRestoredCharacter(t *testing.T) {
	t.Run("restoring is 200", func(t *testing.T) {
		need(t, s.charTakedown)
		td := expectDecode[takedownBody](t, s.api.call(t, http.MethodPost, "/admin/takedowns/"+s.charTakedown+"/restore",
			s.c.token, map[string]string{}), http.StatusOK)
		if td.RestoredAt == nil {
			t.Fatalf("takedown %+v, want restored", td)
		}
	})
	t.Run("the discussion is back and the character is active", func(t *testing.T) {
		need(t, s.charTakedown)
		if !s.publicProfile(t).hasConversation(s.charConv) {
			t.Error("the discussion is not back on the profile")
		}
		if c := s.characters(t).character(s.char1); c == nil || c.Status != "active" || c.RemovedReason != "" {
			t.Errorf("A's character %+v, want active", c)
		}
	})
	t.Run("its report was closed by the takedown", func(t *testing.T) {
		need(t, s.char1)
		if got := s.adminContent(t, "/admin/reports", s.char1); got != nil {
			t.Errorf("open report %+v, want none", got)
		}
	})
}

// checkRemovedDiscussion (D13): a hidden discussion takes no new message
// until it is restored.
func (s *scenario) checkRemovedDiscussion(t *testing.T) {
	t.Run("hiding the discussion is 201", func(t *testing.T) {
		need(t, s.charConv)
		td := expectDecode[takedownBody](t, s.api.call(t, http.MethodPost, "/admin/takedowns", s.c.token,
			map[string]string{"target_kind": "conversation", "target_id": s.charConv, "reason": discussionTakedown}), http.StatusCreated)
		s.charConvRemove = td.ID
	})
	t.Run("posting in it is 409 and stores nothing", func(t *testing.T) {
		need(t, s.charConvRemove)
		s.refusedMessage(t, discussionRemoved)
	})
	t.Run("after restoring, posting gets past the checks", func(t *testing.T) {
		need(t, s.charConvRemove)
		expect(t, s.api.call(t, http.MethodPost, "/admin/takedowns/"+s.charConvRemove+"/restore", s.c.token,
			map[string]string{}), http.StatusOK)
		before, err := messageCount(t.Context(), s.db, s.charConv)
		if err != nil {
			t.Fatal(err)
		}
		// The model client points at a closed port: the reply stream starts,
		// then fails, and the reader's message is undone.
		r := s.api.call(t, http.MethodPost, "/conversations/"+s.charConv+"/messages", s.a.token,
			map[string]string{"content": "e2e: and yet?"})
		body := string(r.body)
		if r.status != http.StatusOK || !strings.Contains(body, `"type":"start"`) ||
			!strings.Contains(body, `"type":"error","error":"the character could not answer right now`) {
			t.Fatalf("want a started stream that ends in the model error, got %s", r)
		}
		if after, err := messageCount(t.Context(), s.db, s.charConv); err != nil || after != before {
			t.Errorf("messages %d → %d (%v); want the unanswered message undone", before, after, err)
		}
	})
}

// adminContent returns the reported target (from /admin/reports) or active
// takedown (from /admin/takedowns) of targetID, or nil.
func (s *scenario) adminContent(t *testing.T, path, targetID string) *adminContentBody {
	t.Helper()
	for _, c := range expectDecode[[]adminContentBody](t, s.api.call(t, http.MethodGet, path, s.c.token, nil), http.StatusOK) {
		if c.TargetID == targetID {
			return &c
		}
	}
	return nil
}
