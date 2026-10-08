package server

import (
	"errors"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/an4eetos/serendipity/api/internal/auth"
	"github.com/an4eetos/serendipity/api/internal/kindle"
	"github.com/an4eetos/serendipity/api/internal/quote"
	"github.com/an4eetos/serendipity/api/internal/store"
)

var usernameRe = regexp.MustCompile(`^[a-z0-9_]{3,30}$`)

// ---------------------------------------------------------------------------
// Profiles
// ---------------------------------------------------------------------------

func (s *Server) getMe(w http.ResponseWriter, r *http.Request) {
	p, err := s.store.GetProfile(r.Context(), auth.UserID(r.Context()))
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, p)
}

func (s *Server) patchMe(w http.ResponseWriter, r *http.Request) {
	var p store.ProfilePatch
	if err := decodeJSON(r, &p); err != nil {
		writeError(w, r, err)
		return
	}
	if p.Username != nil {
		u := strings.ToLower(strings.TrimSpace(*p.Username))
		if !usernameRe.MatchString(u) {
			writeError(w, r, badRequest("username must be 3–30 characters: a–z, 0–9 or _"))
			return
		}
		p.Username = &u
	}
	prof, err := s.store.UpdateProfile(r.Context(), auth.UserID(r.Context()), p)
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		writeError(w, r, apiError{http.StatusConflict, "that username is taken"})
		return
	}
	if errors.As(err, &pgErr) && pgErr.Code == "23514" {
		writeError(w, r, badRequest("invalid profile field"))
		return
	}
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, prof)
}

func (s *Server) getPublicProfile(w http.ResponseWriter, r *http.Request) {
	p, err := s.store.GetProfileByUsername(r.Context(), chi.URLParam(r, "username"))
	if err != nil {
		writeError(w, r, err)
		return
	}
	// Private profiles are visible only to their owner.
	if !p.IsPublic && auth.UserID(r.Context()) != p.ID {
		writeError(w, r, store.ErrNotFound)
		return
	}
	pub, err := s.store.GetPublicProfile(r.Context(), p)
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, pub)
}

// ---------------------------------------------------------------------------
// Works & library
// ---------------------------------------------------------------------------

func (s *Server) searchWorks(w http.ResponseWriter, r *http.Request) {
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	if len(q) < 2 {
		writeError(w, r, badRequest("search query is too short"))
		return
	}
	results, err := s.openLibrary.Search(r.Context(), q)
	if err != nil {
		writeError(w, r, apiError{http.StatusBadGateway, "book search is unavailable right now"})
		return
	}
	writeJSON(w, http.StatusOK, results)
}

type addToLibraryRequest struct {
	OpenLibraryKey *string  `json:"openlibrary_key"`
	Title          string   `json:"title"`
	Authors        []string `json:"authors"`
	ISBN           *string  `json:"isbn"`
	CoverURL       *string  `json:"cover_url"`
	Subjects       []string `json:"subjects"`
}

func (s *Server) addToLibrary(w http.ResponseWriter, r *http.Request) {
	var req addToLibraryRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, r, err)
		return
	}
	req.Title = strings.TrimSpace(req.Title)
	if req.Title == "" || len(req.Title) > 500 {
		writeError(w, r, badRequest("title is required"))
		return
	}
	if req.OpenLibraryKey != nil && !strings.HasPrefix(*req.OpenLibraryKey, "/works/") {
		writeError(w, r, badRequest("invalid Open Library key"))
		return
	}
	if req.CoverURL != nil && !strings.HasPrefix(*req.CoverURL, "https://covers.openlibrary.org/") {
		req.CoverURL = nil
	}
	if len(req.Subjects) > 8 {
		req.Subjects = req.Subjects[:8]
	}
	uid := auth.UserID(r.Context())
	work, err := s.store.EnsureWork(r.Context(), store.Work{
		Title:          req.Title,
		Authors:        req.Authors,
		ISBN:           req.ISBN,
		OpenLibraryKey: req.OpenLibraryKey,
		CoverURL:       req.CoverURL,
		Subjects:       req.Subjects,
	}, uid)
	if err != nil {
		writeError(w, r, err)
		return
	}
	item, err := s.store.AddToLibrary(r.Context(), uid, work.ID)
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, item)
}

func (s *Server) listLibrary(w http.ResponseWriter, r *http.Request) {
	items, err := s.store.ListLibrary(r.Context(), auth.UserID(r.Context()))
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) getLibraryItem(w http.ResponseWriter, r *http.Request) {
	item, err := s.store.GetLibraryItem(r.Context(), auth.UserID(r.Context()), chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (s *Server) patchLibraryItem(w http.ResponseWriter, r *http.Request) {
	var p store.LibraryPatch
	if err := decodeJSON(r, &p); err != nil {
		writeError(w, r, err)
		return
	}
	uid := auth.UserID(r.Context())
	if p.Progress != nil && (*p.Progress < 0 || *p.Progress > 1) {
		writeError(w, r, badRequest("progress must be between 0 and 1"))
		return
	}
	if p.Status != nil && !oneOf(*p.Status, "reading", "finished", "abandoned") {
		writeError(w, r, badRequest("invalid status"))
		return
	}
	if (p.FilePath == nil) != (p.FileType == nil) {
		writeError(w, r, badRequest("file_path and file_type go together"))
		return
	}
	if p.FilePath != nil {
		// Files live in the uploader's own folder of the private bucket.
		if !strings.HasPrefix(*p.FilePath, uid+"/") || strings.Contains(*p.FilePath, "..") {
			writeError(w, r, badRequest("file must be in your own folder"))
			return
		}
		if !oneOf(*p.FileType, "epub", "pdf") {
			writeError(w, r, badRequest("only EPUB and PDF files are supported"))
			return
		}
	}
	item, err := s.store.UpdateLibraryItem(r.Context(), uid, chi.URLParam(r, "id"), p)
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (s *Server) getFileURL(w http.ResponseWriter, r *http.Request) {
	item, err := s.store.GetLibraryItem(r.Context(), auth.UserID(r.Context()), chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, r, err)
		return
	}
	if item.FilePath == nil {
		writeError(w, r, apiError{http.StatusNotFound, "no file uploaded for this book"})
		return
	}
	u, err := s.storage.SignedURL(r.Context(), *item.FilePath, 10*time.Minute)
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"url": u, "file_type": *item.FileType})
}

// ---------------------------------------------------------------------------
// Kindle import
// ---------------------------------------------------------------------------

type importResult struct {
	Books          int `json:"books"`
	Imported       int `json:"imported"`
	Duplicates     int `json:"duplicates"`
	SkippedTooLong int `json:"skipped_too_long"`
	OrphanNotes    int `json:"orphan_notes"`
}

func (s *Server) importKindle(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 20<<20)
	file, _, err := r.FormFile("file")
	if err != nil {
		writeError(w, r, badRequest("upload your My Clippings.txt as the \"file\" field"))
		return
	}
	defer file.Close()
	books, err := kindle.Parse(file)
	if err != nil {
		writeError(w, r, badRequest("could not read the clippings file"))
		return
	}

	ctx, uid := r.Context(), auth.UserID(r.Context())
	var res importResult
	for _, b := range books {
		item, err := s.store.FindLibraryItemByTitle(ctx, uid, b.Title)
		if errors.Is(err, store.ErrNotFound) {
			var authors []string
			if b.Author != "" {
				authors = []string{b.Author}
			}
			var work store.Work
			work, err = s.store.EnsureWork(ctx, store.Work{Title: b.Title, Authors: authors}, uid)
			if err == nil {
				item, err = s.store.AddToLibrary(ctx, uid, work.ID)
			}
		}
		if err != nil {
			writeError(w, r, err)
			return
		}
		res.Books++
		res.OrphanNotes += b.OrphanNotes
		for _, h := range b.Highlights {
			if quote.ValidateText(h.Text) != nil {
				res.SkippedTooLong++
				continue
			}
			inserted, err := s.store.ImportQuote(ctx, uid, store.NewQuote{
				WorkID:        item.Work.ID,
				LibraryItemID: &item.ID,
				Text:          h.Text,
				Location:      h.Location(),
				Note:          h.Note,
				Source:        "kindle",
			})
			if err != nil {
				writeError(w, r, err)
				return
			}
			if inserted {
				res.Imported++
			} else {
				res.Duplicates++
			}
		}
	}
	writeJSON(w, http.StatusOK, res)
}

// ---------------------------------------------------------------------------
// Quotes
// ---------------------------------------------------------------------------

type createQuoteRequest struct {
	LibraryItemID string   `json:"library_item_id"`
	Text          string   `json:"text"`
	Location      string   `json:"location"`
	Anchor        *string  `json:"anchor"`
	Position      *float64 `json:"position"`
	Note          string   `json:"note"`
}

func (s *Server) createQuote(w http.ResponseWriter, r *http.Request) {
	var req createQuoteRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, r, err)
		return
	}
	req.Text = strings.TrimSpace(req.Text)
	if err := quote.ValidateText(req.Text); err != nil {
		writeError(w, r, badRequest("%s", err.Error()))
		return
	}
	if req.Position != nil && (*req.Position < 0 || *req.Position > 1) {
		req.Position = nil
	}
	uid := auth.UserID(r.Context())
	item, err := s.store.GetLibraryItem(r.Context(), uid, req.LibraryItemID)
	if err != nil {
		writeError(w, r, err)
		return
	}
	q, err := s.store.CreateQuote(r.Context(), uid, store.NewQuote{
		WorkID:        item.Work.ID,
		LibraryItemID: &item.ID,
		Text:          req.Text,
		Location:      truncate(strings.TrimSpace(req.Location), 200),
		Anchor:        req.Anchor,
		Position:      req.Position,
		Note:          req.Note,
		Source:        "upload",
	})
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, q)
}

func (s *Server) listQuotes(w http.ResponseWriter, r *http.Request) {
	qs, err := s.store.ListQuotes(r.Context(), auth.UserID(r.Context()), r.URL.Query().Get("work_id"))
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, qs)
}

type quoteWithConversations struct {
	store.Quote
	Conversations []store.Conversation `json:"conversations"`
}

func (s *Server) getQuote(w http.ResponseWriter, r *http.Request) {
	ctx, uid := r.Context(), auth.UserID(r.Context())
	q, err := s.store.GetQuote(ctx, uid, chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, r, err)
		return
	}
	convs, err := s.store.ListConversationsForQuote(ctx, uid, q.ID)
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, quoteWithConversations{Quote: q, Conversations: convs})
}

func (s *Server) patchQuote(w http.ResponseWriter, r *http.Request) {
	var p store.QuotePatch
	if err := decodeJSON(r, &p); err != nil {
		writeError(w, r, err)
		return
	}
	ctx, uid, id := r.Context(), auth.UserID(r.Context()), chi.URLParam(r, "id")
	current, err := s.store.GetQuote(ctx, uid, id)
	if err != nil {
		writeError(w, r, err)
		return
	}
	if p.Visibility != nil && !oneOf(*p.Visibility, "private", "public") {
		writeError(w, r, badRequest("invalid visibility"))
		return
	}
	if p.Note != nil && len(*p.Note) > 5000 {
		writeError(w, r, badRequest("note is too long"))
		return
	}
	// Check the result of the patch against the publishing rules.
	note, visibility := current.Note, current.Visibility
	if p.Note != nil {
		note = *p.Note
	}
	if p.Visibility != nil {
		visibility = *p.Visibility
	}
	if visibility == "public" {
		if err := quote.ValidatePublic(current.Text, note, current.Location); err != nil {
			writeError(w, r, badRequest("%s", err.Error()))
			return
		}
	}
	q, err := s.store.UpdateQuote(ctx, uid, id, p)
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, q)
}

func (s *Server) deleteQuote(w http.ResponseWriter, r *http.Request) {
	if err := s.store.DeleteQuote(r.Context(), auth.UserID(r.Context()), chi.URLParam(r, "id")); err != nil {
		writeError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ---------------------------------------------------------------------------
// Personas
// ---------------------------------------------------------------------------

func (s *Server) listPersonas(w http.ResponseWriter, r *http.Request) {
	ps, err := s.store.ListPersonas(r.Context(), auth.UserID(r.Context()))
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, ps)
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func oneOf(v string, options ...string) bool {
	for _, o := range options {
		if v == o {
			return true
		}
	}
	return false
}

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n])
}
