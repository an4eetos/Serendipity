// Package converters maps catalog values to HTTP responses. Other contexts
// use WorkResponse to embed works in theirs.
package converters

import "github.com/an4eetos/serendipity/backend/internal/catalog/domain"

type WorkResponse struct {
	ID             string   `json:"id"`
	Title          string   `json:"title"`
	Authors        []string `json:"authors"`
	ISBN           *string  `json:"isbn"`
	OpenLibraryKey *string  `json:"openlibrary_key"`
	CoverURL       *string  `json:"cover_url"`
	Subjects       []string `json:"subjects"`
}

func ToWorkResponse(w domain.Work) WorkResponse {
	return WorkResponse{
		ID:             w.ID,
		Title:          w.Title,
		Authors:        nonNil(w.Authors),
		ISBN:           w.ISBN,
		OpenLibraryKey: w.OpenLibraryKey,
		CoverURL:       w.CoverURL,
		Subjects:       nonNil(w.Subjects),
	}
}

type SearchResultResponse struct {
	OpenLibraryKey string   `json:"openlibrary_key"`
	Title          string   `json:"title"`
	Authors        []string `json:"authors"`
	ISBN           *string  `json:"isbn"`
	CoverURL       *string  `json:"cover_url"`
	Subjects       []string `json:"subjects"`
	FirstPublished *int     `json:"first_published"`
}

func ToSearchResultResponses(rs []domain.SearchResult) []SearchResultResponse {
	out := make([]SearchResultResponse, len(rs))
	for i, r := range rs {
		out[i] = SearchResultResponse{
			OpenLibraryKey: r.OpenLibraryKey,
			Title:          r.Title,
			Authors:        nonNil(r.Authors),
			ISBN:           r.ISBN,
			CoverURL:       r.CoverURL,
			Subjects:       nonNil(r.Subjects),
			FirstPublished: r.FirstPublished,
		}
	}
	return out
}

// nonNil keeps empty lists as [] rather than null in JSON.
func nonNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}
