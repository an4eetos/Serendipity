package domain

import (
	"errors"
	"regexp"
	"strings"
	"testing"
	"time"
)

var stamp = time.Date(2026, 11, 16, 12, 0, 0, 0, time.UTC)

func TestCharacterDraftValidate(t *testing.T) {
	valid := CharacterDraft{Name: "Ada Lovelace", Era: "1840s", ShortBio: "Countess.", Prompt: "You are Ada."}
	with := func(f func(*CharacterDraft)) CharacterDraft {
		d := valid
		f(&d)
		return d
	}
	tests := []struct {
		name    string
		draft   CharacterDraft
		want    CharacterDraft
		wantErr error
	}{
		{"valid", valid, valid, nil},
		{"trims every field", CharacterDraft{Name: "  Ada Lovelace  ", Era: "\t1840s ", ShortBio: " Countess.\n", Prompt: "\n You are Ada. \n"}, valid, nil},
		{"era and bio may be empty", with(func(d *CharacterDraft) { d.Era, d.ShortBio = "  ", "" }),
			with(func(d *CharacterDraft) { d.Era, d.ShortBio = "", "" }), nil},
		{"limits counted in characters", CharacterDraft{
			Name: strings.Repeat("é", 80), Era: strings.Repeat("é", 80), ShortBio: strings.Repeat("é", 300), Prompt: strings.Repeat("é", 8000),
		}, CharacterDraft{
			Name: strings.Repeat("é", 80), Era: strings.Repeat("é", 80), ShortBio: strings.Repeat("é", 300), Prompt: strings.Repeat("é", 8000),
		}, nil},
		{"surrounding spaces don't count", with(func(d *CharacterDraft) { d.Name = " " + strings.Repeat("a", 80) + " " }),
			with(func(d *CharacterDraft) { d.Name = strings.Repeat("a", 80) }), nil},
		{"blank name", with(func(d *CharacterDraft) { d.Name = "   " }), CharacterDraft{}, ErrCharacterName},
		{"name of 81", with(func(d *CharacterDraft) { d.Name = strings.Repeat("a", 81) }), CharacterDraft{}, ErrCharacterName},
		{"era of 81", with(func(d *CharacterDraft) { d.Era = strings.Repeat("a", 81) }), CharacterDraft{}, ErrCharacterEra},
		{"bio of 301", with(func(d *CharacterDraft) { d.ShortBio = strings.Repeat("a", 301) }), CharacterDraft{}, ErrCharacterBio},
		{"blank prompt", with(func(d *CharacterDraft) { d.Prompt = " \n\t " }), CharacterDraft{}, ErrCharacterPrompt},
		{"prompt of 8001", with(func(d *CharacterDraft) { d.Prompt = strings.Repeat("a", 8001) }), CharacterDraft{}, ErrCharacterPrompt},
		{"the name is checked first", CharacterDraft{Era: strings.Repeat("a", 81)}, CharacterDraft{}, ErrCharacterName},
		{"then the era", CharacterDraft{Name: "x", Era: strings.Repeat("a", 81), ShortBio: strings.Repeat("a", 301)}, CharacterDraft{}, ErrCharacterEra},
		{"then the bio", CharacterDraft{Name: "x", ShortBio: strings.Repeat("a", 301)}, CharacterDraft{}, ErrCharacterBio},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.draft.Validate()
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v, want %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("Validate() = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestCharacterErrorMessages(t *testing.T) {
	for err, want := range map[error]string{
		ErrCharacterName:       "name must be 1–80 characters",
		ErrCharacterEra:        "era must be at most 80 characters",
		ErrCharacterBio:        "bio must be at most 300 characters",
		ErrCharacterPrompt:     "prompt must be 1–8000 characters",
		ErrNameLocked:          "the name can't change once the character has been in a discussion",
		ErrCharacterLimit:      "you've reached your limit of active characters — archive one first",
		ErrCharacterArchived:   "this character is archived — unarchive it to continue",
		ErrCharacterRemoved:    "a moderator removed this character",
		ErrConversationRemoved: "a moderator removed this discussion — it can't continue",
	} {
		if err.Error() != want {
			t.Errorf("message %q, want %q", err.Error(), want)
		}
	}
}

func custom() Persona {
	return Persona{ID: "p1", Name: "Ada", Era: "1840s", ShortBio: "Countess.", SystemPrompt: "You are Ada.", OwnerID: "u1"}
}

func archived() Persona {
	p := custom()
	p.ArchivedAt = &stamp
	return p
}

func removed() Persona {
	p := custom()
	p.RemovedAt, p.RemovedReason = &stamp, "living person"
	return p
}

func TestPersonaStatus(t *testing.T) {
	both := archived()
	both.RemovedAt = &stamp
	tests := []struct {
		name       string
		p          Persona
		want       PersonaStatus
		wantUsable error
	}{
		{"active", custom(), PersonaActive, nil},
		{"preset", Persona{ID: "s", Name: "Socrates", IsPreset: true}, PersonaActive, nil},
		{"archived", archived(), PersonaArchived, ErrCharacterArchived},
		{"removed", removed(), PersonaRemoved, ErrCharacterRemoved},
		{"removed wins over archived", both, PersonaRemoved, ErrCharacterRemoved},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.p.Status(); got != tt.want {
				t.Errorf("Status() = %q, want %q", got, tt.want)
			}
			if err := tt.p.CheckUsable(); !errors.Is(err, tt.wantUsable) {
				t.Errorf("CheckUsable() = %v, want %v", err, tt.wantUsable)
			}
		})
	}
}

func ptr[T any](v T) *T { return &v }

func TestPersonaEdit(t *testing.T) {
	used := custom()
	used.HasConversations = true
	tests := []struct {
		name    string
		p       Persona
		patch   CharacterPatch
		want    Persona
		wantErr error
	}{
		{"an empty patch changes nothing", custom(), CharacterPatch{}, custom(), nil},
		{"renames an unused character, trimmed", custom(), CharacterPatch{Name: ptr("  Augusta  ")},
			func() Persona { p := custom(); p.Name = "Augusta"; return p }(), nil},
		{"edits era, bio and prompt", custom(), CharacterPatch{Era: ptr(""), ShortBio: ptr("Mathematician."), Prompt: ptr(" New. ")},
			func() Persona {
				p := custom()
				p.Era, p.ShortBio, p.SystemPrompt = "", "Mathematician.", "New."
				return p
			}(), nil},
		{"a used character keeps its name", used, CharacterPatch{Name: ptr("Augusta")}, Persona{}, ErrNameLocked},
		{"a used character may resend the same name", used, CharacterPatch{Name: ptr(" Ada "), Prompt: ptr("New.")},
			func() Persona { p := used; p.SystemPrompt = "New."; return p }(), nil},
		{"a used character's other fields change", used, CharacterPatch{ShortBio: ptr("Mathematician.")},
			func() Persona { p := used; p.ShortBio = "Mathematician."; return p }(), nil},
		{"invalid wins over the lock", used, CharacterPatch{Name: ptr("")}, Persona{}, ErrCharacterName},
		{"blank prompt", custom(), CharacterPatch{Prompt: ptr("  ")}, Persona{}, ErrCharacterPrompt},
		{"an archived character can be edited and stays archived", archived(), CharacterPatch{ShortBio: ptr("x")},
			func() Persona { p := archived(); p.ShortBio = "x"; return p }(), nil},
		{"a removed character can't be edited", removed(), CharacterPatch{ShortBio: ptr("x")}, Persona{}, ErrCharacterRemoved},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.p.Edit(tt.patch)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v, want %v", err, tt.wantErr)
			}
			if !equalPersona(got, tt.want) {
				t.Errorf("Edit() = %#v, want %#v", got, tt.want)
			}
		})
	}
}

// equalPersona compares personas, with time pointers by value.
func equalPersona(a, b Persona) bool {
	sameTime := func(x, y *time.Time) bool { return (x == nil) == (y == nil) && (x == nil || x.Equal(*y)) }
	if !sameTime(a.ArchivedAt, b.ArchivedAt) || !sameTime(a.RemovedAt, b.RemovedAt) {
		return false
	}
	a.ArchivedAt, a.RemovedAt, b.ArchivedAt, b.RemovedAt = nil, nil, nil, nil
	return a == b
}

func TestPersonaArchive(t *testing.T) {
	later := stamp.Add(time.Hour)
	got, err := custom().Archive(later)
	if err != nil || got.ArchivedAt == nil || !got.ArchivedAt.Equal(later) || got.Status() != PersonaArchived {
		t.Errorf("Archive() = %#v, %v", got, err)
	}
	// Archiving again keeps the first time.
	if again, err := archived().Archive(later); err != nil || !again.ArchivedAt.Equal(stamp) {
		t.Errorf("Archive(archived) = %#v, %v; want unchanged", again, err)
	}
	if _, err := removed().Archive(later); !errors.Is(err, ErrCharacterRemoved) {
		t.Errorf("Archive(removed) err = %v", err)
	}

	if got, err := archived().Unarchive(); err != nil || got.ArchivedAt != nil || got.Status() != PersonaActive {
		t.Errorf("Unarchive() = %#v, %v", got, err)
	}
	if got, err := custom().Unarchive(); err != nil || !equalPersona(got, custom()) {
		t.Errorf("Unarchive(active) = %#v, %v; want a no-op", got, err)
	}
	if _, err := removed().Unarchive(); !errors.Is(err, ErrCharacterRemoved) {
		t.Errorf("Unarchive(removed) err = %v", err)
	}
}

func TestNameLocked(t *testing.T) {
	p := custom()
	if p.NameLocked() {
		t.Error("an unused character is name-locked")
	}
	p.HasConversations = true
	if !p.NameLocked() {
		t.Error("a used character isn't name-locked")
	}
}

func TestCharacterLimit(t *testing.T) {
	limit := CharacterLimit(3)
	if err := limit.CheckRoom(2); err != nil {
		t.Errorf("CheckRoom(limit-1) = %v", err)
	}
	for _, active := range []int{3, 4} {
		if err := limit.CheckRoom(active); !errors.Is(err, ErrCharacterLimit) {
			t.Errorf("CheckRoom(%d) = %v, want ErrCharacterLimit", active, err)
		}
	}
	if err := CharacterLimit(1).CheckRoom(0); err != nil {
		t.Errorf("CheckRoom(0) at limit 1 = %v", err)
	}
}

func TestNewCharacterList(t *testing.T) {
	l := NewCharacterList([]Persona{custom(), archived(), removed(), custom()}, 5)
	if l.Active != 2 || l.Limit != 5 || len(l.Characters) != 4 {
		t.Errorf("NewCharacterList() = %+v, want 2 active of 4", l)
	}
}

var slugPattern = regexp.MustCompile(`^[a-z0-9-]+$`)

func TestSlugBase(t *testing.T) {
	tests := []struct{ name, want string }{
		{"Ada Lovelace", "ada-lovelace"},
		{"  Marcus   Aurelius!! ", "marcus-aurelius"},
		{"Hypatia of Alexandria (c. 360)", "hypatia-of-alexandria-c-360"},
		{"Émilie du Châtelet", "milie-du-ch-telet"},
		{"--Ada--", "ada"},
		{"孔子", "character"},
		{"!!!", "character"},
		{"", "character"},
		{strings.Repeat("a", 60), strings.Repeat("a", 48)},
		{strings.Repeat("a", 47) + " b", strings.Repeat("a", 47)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SlugBase(tt.name)
			if got != tt.want {
				t.Errorf("SlugBase(%q) = %q, want %q", tt.name, got, tt.want)
			}
			if len(got) > 48 || !slugPattern.MatchString(got) || strings.HasPrefix(got, "-") || strings.HasSuffix(got, "-") {
				t.Errorf("SlugBase(%q) = %q breaks the slug rules", tt.name, got)
			}
		})
	}
}

// R4 (exit criterion): a custom character gets the same shared rules as a
// preset, under its own name, after its own prompt.
func TestCustomCharacterPromptGetsTheSharedRules(t *testing.T) {
	ada := custom()
	ada.SystemPrompt = "  You are Ada Lovelace, writing in 1843.  "
	preset := Persona{Name: "Diogenes", SystemPrompt: "You are Diogenes.", IsPreset: true}
	passage := Passage{Text: "x", Title: "t", Reaction: "r"}

	got := NewCharacterPrompt(ada, passage, nil).System
	if !strings.HasPrefix(got, "You are Ada Lovelace, writing in 1843.\n") {
		t.Errorf("system prompt should start with the custom prompt: %q", got[:60])
	}
	rules := strings.TrimPrefix(NewCharacterPrompt(preset, passage, nil).System, "You are Diogenes.")
	if want := strings.ReplaceAll(rules, "Diogenes", "Ada"); !strings.HasSuffix(got, want) {
		t.Errorf("custom system prompt doesn't end with the shared rules:\n%s", got)
	}
	for _, want := range []string{"Stay in character as Ada", "the way Ada genuinely would", "Never quote more than a single sentence"} {
		if !strings.Contains(got, want) {
			t.Errorf("custom system prompt missing %q", want)
		}
	}
}
