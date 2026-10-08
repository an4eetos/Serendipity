// Package openlibrary searches the Open Library catalog for book metadata.
package openlibrary

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

type Client struct {
	http    *http.Client
	baseURL string
}

func New() *Client {
	return &Client{http: &http.Client{Timeout: 10 * time.Second}, baseURL: "https://openlibrary.org"}
}

// Result is a catalog entry, shaped like the API's work fields.
type Result struct {
	OpenLibraryKey string   `json:"openlibrary_key"`
	Title          string   `json:"title"`
	Authors        []string `json:"authors"`
	ISBN           *string  `json:"isbn"`
	CoverURL       *string  `json:"cover_url"`
	Subjects       []string `json:"subjects"`
	FirstPublished *int     `json:"first_published"`
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

const maxSubjects = 8

func (c *Client) Search(ctx context.Context, q string) ([]Result, error) {
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
	out := make([]Result, 0, len(sr.Docs))
	for _, d := range sr.Docs {
		if d.Key == "" || d.Title == "" {
			continue
		}
		r := Result{
			OpenLibraryKey: d.Key,
			Title:          d.Title,
			Authors:        d.AuthorName,
			Subjects:       d.Subject,
			FirstPublished: d.FirstPublishYear,
		}
		if r.Authors == nil {
			r.Authors = []string{}
		}
		if len(r.Subjects) > maxSubjects {
			r.Subjects = r.Subjects[:maxSubjects]
		}
		if r.Subjects == nil {
			r.Subjects = []string{}
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
