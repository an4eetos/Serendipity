//go:build e2e

package e2e

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strings"
	"testing"
)

// apiClient calls the Serendipity API under test.
type apiClient struct {
	base string
	hc   *http.Client
}

// response is a finished API call.
type response struct {
	method string
	path   string
	status int
	body   []byte
}

func (r response) String() string {
	return fmt.Sprintf("%s %s → HTTP %d %s", r.method, r.path, r.status, snippet(r.body))
}

// call sends a request; body, when not nil, is sent as JSON. A []byte body
// is sent as is. token, when set, is the bearer token.
func (c *apiClient) call(t *testing.T, method, path, token string, body any) response {
	t.Helper()
	var rd io.Reader
	if body != nil {
		raw, ok := body.([]byte)
		if !ok {
			var err error
			if raw, err = json.Marshal(body); err != nil {
				t.Fatal(err)
			}
		}
		rd = bytes.NewReader(raw)
	}
	req, err := http.NewRequestWithContext(t.Context(), method, c.base+path, rd)
	if err != nil {
		t.Fatal(err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	return c.send(t, req, token)
}

// importKindle posts a My Clippings.txt file. The ownership_confirmed field
// is sent only when confirm is true.
func (c *apiClient) importKindle(t *testing.T, token, clippings string, confirm bool) response {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, err := mw.CreateFormFile("file", "My Clippings.txt")
	if err == nil {
		_, err = io.WriteString(fw, clippings)
	}
	if err == nil && confirm {
		err = mw.WriteField("ownership_confirmed", "true")
	}
	if err == nil {
		err = mw.Close()
	}
	if err != nil {
		t.Fatal(err)
	}
	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, c.base+"/imports/kindle", &buf)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", mw.FormDataContentType())
	return c.send(t, req, token)
}

func (c *apiClient) send(t *testing.T, req *http.Request, token string) response {
	t.Helper()
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := c.hc.Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", req.Method, req.URL.Path, err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("%s %s: read body: %v", req.Method, req.URL.Path, err)
	}
	return response{method: req.Method, path: req.URL.RequestURI(), status: resp.StatusCode, body: raw}
}

// expect stops the check unless r has the wanted status.
func expect(t *testing.T, r response, want int) {
	t.Helper()
	if r.status != want {
		t.Fatalf("want HTTP %d, got %s", want, r)
	}
}

// expectError stops the check unless r has the wanted status and an error
// message starting with msg.
func expectError(t *testing.T, r response, want int, msg string) {
	t.Helper()
	expect(t, r, want)
	var e struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal(r.body, &e); err != nil || !strings.HasPrefix(e.Error, msg) {
		t.Fatalf("want error %q, got %s", msg, r)
	}
}

// decode parses r's JSON body into a T.
func decode[T any](t *testing.T, r response) T {
	t.Helper()
	var v T
	if err := json.Unmarshal(r.body, &v); err != nil {
		t.Fatalf("decode %T: %v (%s)", v, err, r)
	}
	return v
}

// expectDecode checks the status and decodes the body.
func expectDecode[T any](t *testing.T, r response, want int) T {
	t.Helper()
	expect(t, r, want)
	return decode[T](t, r)
}

type versionBody struct {
	Version string `json:"version"`
}

type meBody struct {
	ID                  string  `json:"id"`
	Username            string  `json:"username"`
	IsPublic            bool    `json:"is_public"`
	TermsVersion        string  `json:"terms_version"`
	TermsAcceptedAt     *string `json:"terms_accepted_at"`
	CurrentTermsVersion string  `json:"current_terms_version"`
	TermsAccepted       bool    `json:"terms_accepted"`
}

type workBody struct {
	ID    string `json:"id"`
	Title string `json:"title"`
}

type libraryItemBody struct {
	ID                   string   `json:"id"`
	Work                 workBody `json:"work"`
	FilePath             *string  `json:"file_path"`
	FileType             *string  `json:"file_type"`
	OwnershipConfirmedAt *string  `json:"ownership_confirmed_at"`
}

type importReportBody struct {
	Books          int `json:"books"`
	Imported       int `json:"imported"`
	Duplicates     int `json:"duplicates"`
	SkippedTooLong int `json:"skipped_too_long"`
}

type messageBody struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type personaBody struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Era      string `json:"era"`
	ShortBio string `json:"short_bio"`
	IsPreset bool   `json:"is_preset"`
	Status   string `json:"status"`
}

type conversationBody struct {
	ID            string        `json:"id"`
	QuoteID       string        `json:"quote_id"`
	Persona       personaBody   `json:"persona"`
	Messages      []messageBody `json:"messages"`
	RemovedAt     *string       `json:"removed_at"`
	RemovedReason string        `json:"removed_reason"`
}

// quoteBody is a Quote, or a Thread when Conversations is present.
type quoteBody struct {
	ID            string             `json:"id"`
	WorkID        string             `json:"work_id"`
	Text          string             `json:"text"`
	Location      string             `json:"location"`
	Position      *float64           `json:"position"`
	Reaction      string             `json:"reaction"`
	Source        string             `json:"source"`
	Visibility    string             `json:"visibility"`
	PassageHidden bool               `json:"passage_hidden"`
	RemovedAt     *string            `json:"removed_at"`
	RemovedReason string             `json:"removed_reason"`
	Conversations []conversationBody `json:"conversations"`
}

type trailPointBody struct {
	QuoteID  string  `json:"quote_id"`
	Position float64 `json:"position"`
}

type publicBookBody struct {
	Work         workBody `json:"work"`
	PublicQuotes int      `json:"public_quotes"`
	Trail        struct {
		Points   []trailPointBody `json:"points"`
		Unplaced int              `json:"unplaced"`
	} `json:"trail"`
}

type publicProfileBody struct {
	Profile meBody           `json:"profile"`
	Books   []publicBookBody `json:"books"`
	Quotes  []quoteBody      `json:"quotes"`
}

// quote returns the listed quote with id, or nil.
func (p publicProfileBody) quote(id string) *quoteBody {
	for i := range p.Quotes {
		if p.Quotes[i].ID == id {
			return &p.Quotes[i]
		}
	}
	return nil
}

// book returns the book of workID, or nil.
func (p publicProfileBody) book(workID string) *publicBookBody {
	for i := range p.Books {
		if p.Books[i].Work.ID == workID {
			return &p.Books[i]
		}
	}
	return nil
}

// hasConversation reports whether any listed quote shows conversation id.
func (p publicProfileBody) hasConversation(id string) bool {
	return p.conversation(id) != nil
}

// conversation returns the listed conversation with id, or nil.
func (p publicProfileBody) conversation(id string) *conversationBody {
	for i := range p.Quotes {
		for j := range p.Quotes[i].Conversations {
			if p.Quotes[i].Conversations[j].ID == id {
				return &p.Quotes[i].Conversations[j]
			}
		}
	}
	return nil
}

type reportBody struct {
	ID         string `json:"id"`
	TargetKind string `json:"target_kind"`
	TargetID   string `json:"target_id"`
	Status     string `json:"status"`
}

type reportedTargetBody struct {
	TargetKind    string `json:"target_kind"`
	TargetID      string `json:"target_id"`
	OwnerID       string `json:"owner_id"`
	OwnerUsername string `json:"owner_username"`
	Reports       []struct {
		ReporterID string `json:"reporter_id"`
	} `json:"reports"`
}

type takedownBody struct {
	ID         string  `json:"id"`
	TargetKind string  `json:"target_kind"`
	TargetID   string  `json:"target_id"`
	OwnerID    string  `json:"owner_id"`
	Reason     string  `json:"reason"`
	HiddenBy   *string `json:"hidden_by"`
	RestoredAt *string `json:"restored_at"`
	RestoredBy *string `json:"restored_by"`
}

// characterBody is a character as its owner, or an admin, sees it.
type characterBody struct {
	ID            string  `json:"id"`
	Slug          string  `json:"slug"`
	Name          string  `json:"name"`
	Era           string  `json:"era"`
	ShortBio      string  `json:"short_bio"`
	Prompt        string  `json:"prompt"`
	Status        string  `json:"status"`
	NameLocked    bool    `json:"name_locked"`
	ArchivedAt    *string `json:"archived_at"`
	RemovedAt     *string `json:"removed_at"`
	RemovedReason string  `json:"removed_reason"`
}

type characterListBody struct {
	Characters  []characterBody `json:"characters"`
	Active      int             `json:"active"`
	ActiveLimit int             `json:"active_limit"`
}

// character returns the listed character with id, or nil.
func (l characterListBody) character(id string) *characterBody {
	for i := range l.Characters {
		if l.Characters[i].ID == id {
			return &l.Characters[i]
		}
	}
	return nil
}

// adminContentBody is a reported or hidden target with the character it
// embeds, prompt included.
type adminContentBody struct {
	ID         string         `json:"id"`
	TargetKind string         `json:"target_kind"`
	TargetID   string         `json:"target_id"`
	OwnerID    string         `json:"owner_id"`
	Character  *characterBody `json:"character"`
	Reports    []struct {
		ReporterID string `json:"reporter_id"`
		Reason     string `json:"reason"`
	} `json:"reports"`
}

type dismissedBody struct {
	Dismissed int `json:"dismissed"`
}
