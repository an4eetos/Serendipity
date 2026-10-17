// Package openlibrary implements ports.Catalog on the Open Library search
// API.
package openlibrary

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/an4eetos/serendipity/backend/internal/catalog/domain"
)

const DefaultBaseURL = "https://openlibrary.org"

type Client struct {
	http    *http.Client
	baseURL string
}

func New(baseURL string) *Client {
	return &Client{http: &http.Client{Timeout: 10 * time.Second}, baseURL: baseURL}
}

type searchResponse struct {
	Docs []struct {
		Key              string   `json:"key"`
		Title            string   `json:"title"`
		AuthorName       []string `json:"author_name"`
		ISBN             []string `json:"isbn"`
		CoverI           *int     `json:"cover_i"`
		Subject          []string `json:"subject"`
		FirstPublishYear *int     `json:"first_publish_year"`
	} `json:"docs"`
}

func (c *Client) Search(ctx context.Context, q string) ([]domain.SearchResult, error) {
	u := c.baseURL + "/search.json?" + url.Values{
		"q":      {q},
		"limit":  {"12"},
		"fields": {"key,title,author_name,isbn,cover_i,subject,first_publish_year"},
	}.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Serendipity/0.1 (reading platform)")
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("open library search: %s", resp.Status)
	}
	var sr searchResponse
	if err := json.NewDecoder(resp.Body).Decode(&sr); err != nil {
		return nil, err
	}
	out := make([]domain.SearchResult, 0, len(sr.Docs))
	for _, d := range sr.Docs {
		if d.Key == "" || d.Title == "" {
			continue
		}
		r := domain.SearchResult{
			OpenLibraryKey: d.Key,
			Title:          d.Title,
			Authors:        d.AuthorName,
			Subjects:       d.Subject,
			FirstPublished: d.FirstPublishYear,
		}
		if len(d.ISBN) > 0 {
			r.ISBN = &d.ISBN[0]
		}
		if d.CoverI != nil {
			cover := fmt.Sprintf("https://covers.openlibrary.org/b/id/%d-M.jpg", *d.CoverI)
			r.CoverURL = &cover
		}
		out = append(out, r)
	}
	return out, nil
}
