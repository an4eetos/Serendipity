package converters

import (
	"time"

	"github.com/an4eetos/serendipity/backend/internal/discussion/domain"
)

// CharacterRequest is a new character as its owner typed it.
type CharacterRequest struct {
	Name     string `json:"name"`
	Era      string `json:"era"`
	ShortBio string `json:"short_bio"`
	Prompt   string `json:"prompt"`
}

// ToDraft is the request as a domain draft, not yet validated.
func (r CharacterRequest) ToDraft() domain.CharacterDraft {
	return domain.CharacterDraft{Name: r.Name, Era: r.Era, ShortBio: r.ShortBio, Prompt: r.Prompt}
}

// CharacterPatchRequest changes the fields that are present; omitted fields
// stay as they are.
type CharacterPatchRequest struct {
	Name     *string `json:"name"`
	Era      *string `json:"era"`
	ShortBio *string `json:"short_bio"`
	Prompt   *string `json:"prompt"`
}

// ToPatch is the request as a domain patch: nil fields stay unchanged.
func (r CharacterPatchRequest) ToPatch() domain.CharacterPatch {
	return domain.CharacterPatch{Name: r.Name, Era: r.Era, ShortBio: r.ShortBio, Prompt: r.Prompt}
}

// CharacterResponse is a reader's own character as its owner sees it. It
// carries the prompt: never embed it in anything another reader can get.
type CharacterResponse struct {
	ID            string     `json:"id"`
	Slug          string     `json:"slug"`
	Name          string     `json:"name"`
	Era           string     `json:"era"`
	ShortBio      string     `json:"short_bio"`
	Prompt        string     `json:"prompt"`
	Status        string     `json:"status"`
	NameLocked    bool       `json:"name_locked"`
	ArchivedAt    *time.Time `json:"archived_at"`
	RemovedAt     *time.Time `json:"removed_at"`
	RemovedReason string     `json:"removed_reason"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}

// ToCharacterResponse is p as its owner sees it, prompt included.
func ToCharacterResponse(p domain.Persona) CharacterResponse {
	return CharacterResponse{
		ID:            p.ID,
		Slug:          p.Slug,
		Name:          p.Name,
		Era:           p.Era,
		ShortBio:      p.ShortBio,
		Prompt:        p.SystemPrompt,
		Status:        string(p.Status()),
		NameLocked:    p.NameLocked(),
		ArchivedAt:    p.ArchivedAt,
		RemovedAt:     p.RemovedAt,
		RemovedReason: p.RemovedReason,
		CreatedAt:     p.CreatedAt,
		UpdatedAt:     p.UpdatedAt,
	}
}

// CharacterListResponse is a reader's own characters with their cap.
type CharacterListResponse struct {
	Characters  []CharacterResponse `json:"characters"` // never null
	Active      int                 `json:"active"`
	ActiveLimit int                 `json:"active_limit"`
}

// ToCharacterListResponse converts l; Characters is never null.
func ToCharacterListResponse(l domain.CharacterList) CharacterListResponse {
	cs := make([]CharacterResponse, len(l.Characters))
	for i, p := range l.Characters {
		cs[i] = ToCharacterResponse(p)
	}
	return CharacterListResponse{Characters: cs, Active: l.Active, ActiveLimit: int(l.Limit)}
}
