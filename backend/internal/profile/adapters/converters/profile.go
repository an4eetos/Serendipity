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

// MeResponse is the signed-in reader's own profile and where they stand on
// the terms of service.
type MeResponse struct {
	ProfileResponse
	TermsVersion        string     `json:"terms_version"`
	TermsAcceptedAt     *time.Time `json:"terms_accepted_at"`
	CurrentTermsVersion string     `json:"current_terms_version"`
	TermsAccepted       bool       `json:"terms_accepted"`
}

func ToMeResponse(m domain.Me) MeResponse {
	return MeResponse{
		ProfileResponse:     ToProfileResponse(m.Profile),
		TermsVersion:        m.Profile.TermsVersion,
		TermsAcceptedAt:     m.Profile.TermsAcceptedAt,
		CurrentTermsVersion: m.CurrentTerms.Version,
		TermsAccepted:       m.TermsAccepted,
	}
}

type TermsResponse struct {
	Version string `json:"version"`
}

func ToTermsResponse(t domain.Terms) TermsResponse {
	return TermsResponse{Version: t.Version}
}

// AcceptTermsRequest names the version of the terms the reader was shown.
type AcceptTermsRequest struct {
	Version string `json:"version"`
}

type CategoryResponse struct {
	Name  string `json:"name"`
	Books int    `json:"books"`
}

type TrailPointResponse struct {
	QuoteID  string  `json:"quote_id"`
	Position float64 `json:"position"`
}

// TrailResponse is a book's reading trail. Points is never null.
type TrailResponse struct {
	Points   []TrailPointResponse `json:"points"`
	Unplaced int                  `json:"unplaced"`
}

func ToTrailResponse(b domain.PublicBook) TrailResponse {
	points := make([]TrailPointResponse, len(b.Trail))
	for i, p := range b.Trail {
		points[i] = TrailPointResponse{QuoteID: p.QuoteID, Position: p.Position}
	}
	return TrailResponse{Points: points, Unplaced: b.Unplaced()}
}

type PublicBookResponse struct {
	Work         catalogconv.WorkResponse `json:"work"`
	Status       string                   `json:"status"`
	Progress     float64                  `json:"progress"`
	PublicQuotes int                      `json:"public_quotes"`
	Trail        TrailResponse            `json:"trail"`
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
			Trail:        ToTrailResponse(b),
		}
	}
	for i, t := range p.Quotes {
		out.Quotes[i] = discussionconv.ToThreadResponse(t)
	}
	return out
}
