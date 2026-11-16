package domain

import (
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	kernel "github.com/an4eetos/serendipity/backend/internal/kernel/domain"
)

// Limits of a reader's own character, in characters after trimming.
const (
	// MaxCharacterNameLength caps a character's name.
	MaxCharacterNameLength = 80
	// MaxCharacterEraLength caps a character's era.
	MaxCharacterEraLength = 80
	// MaxCharacterBioLength caps a character's short bio.
	MaxCharacterBioLength = 300
	// MaxCharacterPromptLength caps a character's prompt.
	MaxCharacterPromptLength = 8000
	maxSlugBaseLength        = 48 // + "-" + 10 hex ≤ 59 < 60 (personas.slug check)
	defaultSlugBase          = "character"
)

// Errors of a reader's own characters.
var (
	// ErrCharacterName is returned for a blank or too long name.
	ErrCharacterName = kernel.Invalid("name must be 1–" + strconv.Itoa(MaxCharacterNameLength) + " characters")
	// ErrCharacterEra is returned for a too long era.
	ErrCharacterEra = kernel.Invalid("era must be at most " + strconv.Itoa(MaxCharacterEraLength) + " characters")
	// ErrCharacterBio is returned for a too long short bio.
	ErrCharacterBio = kernel.Invalid("bio must be at most " + strconv.Itoa(MaxCharacterBioLength) + " characters")
	// ErrCharacterPrompt is returned for a blank or too long prompt.
	ErrCharacterPrompt = kernel.Invalid("prompt must be 1–" + strconv.Itoa(MaxCharacterPromptLength) + " characters")
	// ErrNameLocked is returned for a rename of a character that has been in
	// a discussion.
	ErrNameLocked = kernel.Conflict("the name can't change once the character has been in a discussion")
	// ErrCharacterLimit is returned when the reader has no room for another
	// active character.
	ErrCharacterLimit = kernel.Conflict("you've reached your limit of active characters — archive one first")
	// ErrCharacterArchived is returned for using an archived character.
	ErrCharacterArchived = kernel.Conflict("this character is archived — unarchive it to continue")
	// ErrCharacterRemoved is returned for using or changing a character a
	// moderator removed.
	ErrCharacterRemoved = kernel.Conflict("a moderator removed this character")
)

// PersonaStatus is where a character stands.
type PersonaStatus string

const (
	// PersonaActive is a character that is neither archived nor removed.
	PersonaActive PersonaStatus = "active"
	// PersonaArchived is a character its owner archived.
	PersonaArchived PersonaStatus = "archived"
	// PersonaRemoved is a character a moderator removed.
	PersonaRemoved PersonaStatus = "removed"
)

// Persona is a character readers discuss their reactions with: a preset or
// a reader's own.
type Persona struct {
	ID       string
	Slug     string
	Name     string
	Era      string
	ShortBio string
	// SystemPrompt describes the character to the model. Only its owner and
	// admins ever see it.
	SystemPrompt string
	IsPreset     bool
	OwnerID      string // "" for presets
	ArchivedAt   *time.Time
	RemovedAt    *time.Time
	// RemovedReason tells the owner why a moderator removed the character.
	RemovedReason string
	// HasConversations is set by CharacterRepository reads only; elsewhere it
	// is false and means nothing.
	HasConversations bool
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

// Archived reports whether the owner archived the character.
func (p Persona) Archived() bool {
	return p.ArchivedAt != nil
}

// Removed reports whether a moderator removed the character.
func (p Persona) Removed() bool {
	return p.RemovedAt != nil
}

// Status is removed, else archived, else active.
func (p Persona) Status() PersonaStatus {
	if p.Removed() {
		return PersonaRemoved
	}
	if p.Archived() {
		return PersonaArchived
	}
	return PersonaActive
}

// NameLocked reports whether the name can no longer change: the character
// has been in a discussion.
func (p Persona) NameLocked() bool {
	return p.HasConversations
}

// CheckUsable reports whether the character may start a conversation or take
// a new message.
func (p Persona) CheckUsable() error {
	if p.Removed() {
		return ErrCharacterRemoved
	}
	if p.Archived() {
		return ErrCharacterArchived
	}
	return nil
}

// Edit applies a patch: it merges the set fields over p, validates the
// result and refuses a rename while the name is locked. It returns the
// updated copy; ArchivedAt is untouched.
func (p Persona) Edit(patch CharacterPatch) (Persona, error) {
	if p.Removed() {
		return Persona{}, ErrCharacterRemoved
	}
	d := CharacterDraft{Name: p.Name, Era: p.Era, ShortBio: p.ShortBio, Prompt: p.SystemPrompt}
	if patch.Name != nil {
		d.Name = *patch.Name
	}
	if patch.Era != nil {
		d.Era = *patch.Era
	}
	if patch.ShortBio != nil {
		d.ShortBio = *patch.ShortBio
	}
	if patch.Prompt != nil {
		d.Prompt = *patch.Prompt
	}
	d, err := d.Validate()
	if err != nil {
		return Persona{}, err
	}
	if d.Name != p.Name && p.NameLocked() {
		return Persona{}, ErrNameLocked
	}
	p.Name, p.Era, p.ShortBio, p.SystemPrompt = d.Name, d.Era, d.ShortBio, d.Prompt
	return p, nil
}

// Archive hides the character from the picker as of at. An archived
// character is returned unchanged.
func (p Persona) Archive(at time.Time) (Persona, error) {
	if p.Removed() {
		return Persona{}, ErrCharacterRemoved
	}
	if p.Archived() {
		return p, nil
	}
	p.ArchivedAt = &at
	return p, nil
}

// Unarchive makes the character usable again; it is a no-op for an active
// one.
func (p Persona) Unarchive() (Persona, error) {
	if p.Removed() {
		return Persona{}, ErrCharacterRemoved
	}
	p.ArchivedAt = nil
	return p, nil
}

// CharacterDraft is a reader's character as typed.
type CharacterDraft struct {
	Name, Era, ShortBio, Prompt string
}

// Validate trims every field and checks, in order, the name, era, bio and
// prompt lengths in characters. It returns the trimmed draft.
func (d CharacterDraft) Validate() (CharacterDraft, error) {
	d = CharacterDraft{
		Name:     strings.TrimSpace(d.Name),
		Era:      strings.TrimSpace(d.Era),
		ShortBio: strings.TrimSpace(d.ShortBio),
		Prompt:   strings.TrimSpace(d.Prompt),
	}
	if n := utf8.RuneCountInString(d.Name); n == 0 || n > MaxCharacterNameLength {
		return CharacterDraft{}, ErrCharacterName
	}
	if utf8.RuneCountInString(d.Era) > MaxCharacterEraLength {
		return CharacterDraft{}, ErrCharacterEra
	}
	if utf8.RuneCountInString(d.ShortBio) > MaxCharacterBioLength {
		return CharacterDraft{}, ErrCharacterBio
	}
	if n := utf8.RuneCountInString(d.Prompt); n == 0 || n > MaxCharacterPromptLength {
		return CharacterDraft{}, ErrCharacterPrompt
	}
	return d, nil
}

// CharacterPatch changes the fields that are set.
type CharacterPatch struct {
	Name, Era, ShortBio, Prompt *string
}

// CharacterLimit is how many active custom characters a reader may have.
type CharacterLimit int

// CheckRoom reports whether a reader with active characters may have one
// more.
func (l CharacterLimit) CheckRoom(active int) error {
	if active >= int(l) {
		return ErrCharacterLimit
	}
	return nil
}

// CharacterList is a reader's own characters with their cap.
type CharacterList struct {
	Characters []Persona
	Active     int // characters whose Status() is PersonaActive
	Limit      CharacterLimit
}

// NewCharacterList counts the active characters of cs.
func NewCharacterList(cs []Persona, limit CharacterLimit) CharacterList {
	active := 0
	for _, c := range cs {
		if c.Status() == PersonaActive {
			active++
		}
	}
	return CharacterList{Characters: cs, Active: active, Limit: limit}
}

// ModeratedPersona is a custom character as moderators see it.
type ModeratedPersona struct {
	Persona Persona
	// InPublicDiscussion reports whether at least one discussion public pages
	// would show uses it (quote public and not removed, conversation not
	// removed and answered). Profile visibility is not considered.
	InPublicDiscussion bool
}

// SlugBase is the readable part of a new character's slug: lower-case ASCII
// letters and digits of name, every other run of characters as one "-",
// trimmed of "-", cut to 48 bytes (trailing "-" trimmed again), or
// "character" when nothing is left.
func SlugBase(name string) string {
	var b strings.Builder
	dash := false // the last character written is a "-"
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			dash = false
		case r >= 'A' && r <= 'Z':
			b.WriteRune(r - 'A' + 'a')
			dash = false
		default:
			if !dash {
				b.WriteByte('-')
				dash = true
			}
		}
	}
	base := strings.Trim(b.String(), "-")
	if len(base) > maxSlugBaseLength {
		base = strings.TrimRight(base[:maxSlugBaseLength], "-")
	}
	if base == "" {
		return defaultSlugBase
	}
	return base
}
