// Package converters maps profile HTTP requests and responses to domain
// values.
package converters

import (
	"time"

	catalogconv "github.com/an4eetos/serendipity/backend/internal/catalog/adapters/converters"
	discussionconv "github.com/an4eetos/serendipity/backend/internal/discussion/adapters/converters"
	"github.com/an4eetos/serendipity/backend/internal/profile/domain"
)

type PatchRequest struct {
	Username    *string `json:"username"`
	DisplayName *string `json:"display_name"`
	Bio         *string `json:"bio"`
	IsPublic    *bool   `json:"is_public"`
}

func (r PatchRequest) ToPatch() domain.Patch {
	return domain.Patch{Username: r.Username, DisplayName: r.DisplayName, Bio: r.Bio, IsPublic: r.IsPublic}
}

type ProfileResponse struct {
	ID          string    `json:"id"`
	Username    string    `json:"username"`
	DisplayName string    `json:"display_name"`
	Bio         string    `json:"bio"`
	IsPublic    bool      `json:"is_public"`
	CreatedAt   time.Time `json:"created_at"`
}

func ToProfileResponse(p domain.Profile) ProfileResponse {
	return ProfileResponse{
		ID:          p.ID,
		Username:    p.Username,
		DisplayName: p.DisplayName,
		Bio:         p.Bio,
		IsPublic:    p.IsPublic,
		CreatedAt:   p.CreatedAt,
	}
}

type CategoryResponse struct {
	Name  string `json:"name"`
	Books int    `json:"books"`
}

type PublicBookResponse struct {
	Work         catalogconv.WorkResponse `json:"work"`
	Status       string                   `json:"status"`
	Progress     float64                  `json:"progress"`
	PublicQuotes int                      `json:"public_quotes"`
}

type PublicProfileResponse struct {
	Profile    ProfileResponse                 `json:"profile"`
	Categories []CategoryResponse              `json:"categories"`
	Books      []PublicBookResponse            `json:"books"`
	Quotes     []discussionconv.ThreadResponse `json:"quotes"`
}

func ToPublicProfileResponse(p domain.PublicProfile) PublicProfileResponse {
	out := PublicProfileResponse{
		Profile:    ToProfileResponse(p.Profile),
		Categories: make([]CategoryResponse, len(p.Categories)),
		Books:      make([]PublicBookResponse, len(p.Books)),
		Quotes:     make([]discussionconv.ThreadResponse, len(p.Quotes)),
	}
	for i, c := range p.Categories {
		out.Categories[i] = CategoryResponse{Name: c.Name, Books: c.Books}
	}
	for i, b := range p.Books {
		out.Books[i] = PublicBookResponse{
			Work:         catalogconv.ToWorkResponse(b.Work),
			Status:       string(b.Status),
			Progress:     b.Progress,
			PublicQuotes: b.PublicQuotes,
		}
	}
	for i, t := range p.Quotes {
		out.Quotes[i] = discussionconv.ToThreadResponse(t)
	}
	return out
}
