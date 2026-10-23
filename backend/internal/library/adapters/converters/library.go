// Package converters maps library HTTP requests and responses to domain
// values.
package converters

import (
	"time"

	catalogconv "github.com/an4eetos/serendipity/backend/internal/catalog/adapters/converters"
	catalog "github.com/an4eetos/serendipity/backend/internal/catalog/domain"
	"github.com/an4eetos/serendipity/backend/internal/library/domain"
)

type AddBookRequest struct {
	OpenLibraryKey *string  `json:"openlibrary_key"`
	Title          string   `json:"title"`
	Authors        []string `json:"authors"`
	ISBN           *string  `json:"isbn"`
	CoverURL       *string  `json:"cover_url"`
	Subjects       []string `json:"subjects"`
}

func (r AddBookRequest) ToWork() catalog.Work {
	return catalog.Work{
		Title:          r.Title,
		Authors:        r.Authors,
		ISBN:           r.ISBN,
		OpenLibraryKey: r.OpenLibraryKey,
		CoverURL:       r.CoverURL,
		Subjects:       r.Subjects,
	}
}

type PatchRequest struct {
	Progress           *float64 `json:"progress"`
	Status             *string  `json:"status"`
	LastLocation       *string  `json:"last_location"`
	FilePath           *string  `json:"file_path"`
	FileType           *string  `json:"file_type"`
	OwnershipConfirmed bool     `json:"ownership_confirmed"`
}

func (r PatchRequest) ToPatch() domain.Patch {
	p := domain.Patch{
		Progress:           r.Progress,
		LastLocation:       r.LastLocation,
		FilePath:           r.FilePath,
		OwnershipConfirmed: r.OwnershipConfirmed,
	}
	if r.Status != nil {
		s := domain.ReadingStatus(*r.Status)
		p.Status = &s
	}
	if r.FileType != nil {
		ft := domain.FileType(*r.FileType)
		p.FileType = &ft
	}
	return p
}

type ItemResponse struct {
	ID                   string                   `json:"id"`
	Work                 catalogconv.WorkResponse `json:"work"`
	FilePath             *string                  `json:"file_path"`
	FileType             *string                  `json:"file_type"`
	Progress             float64                  `json:"progress"`
	Status               string                   `json:"status"`
	LastLocation         *string                  `json:"last_location"`
	QuoteCount           int                      `json:"quote_count"`
	OwnershipConfirmedAt *time.Time               `json:"ownership_confirmed_at"`
	CreatedAt            time.Time                `json:"created_at"`
	UpdatedAt            time.Time                `json:"updated_at"`
}

func ToItemResponse(it domain.Item) ItemResponse {
	resp := ItemResponse{
		ID:                   it.ID,
		Work:                 catalogconv.ToWorkResponse(it.Work),
		FilePath:             it.FilePath,
		Progress:             it.Progress,
		Status:               string(it.Status),
		LastLocation:         it.LastLocation,
		QuoteCount:           it.QuoteCount,
		OwnershipConfirmedAt: it.OwnershipConfirmedAt,
		CreatedAt:            it.CreatedAt,
		UpdatedAt:            it.UpdatedAt,
	}
	if it.FileType != nil {
		ft := string(*it.FileType)
		resp.FileType = &ft
	}
	return resp
}

func ToItemResponses(items []domain.Item) []ItemResponse {
	out := make([]ItemResponse, len(items))
	for i, it := range items {
		out[i] = ToItemResponse(it)
	}
	return out
}

type FileLinkResponse struct {
	URL      string `json:"url"`
	FileType string `json:"file_type"`
}

func ToFileLinkResponse(l domain.FileLink) FileLinkResponse {
	return FileLinkResponse{URL: l.URL, FileType: string(l.FileType)}
}
