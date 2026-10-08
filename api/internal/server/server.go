// Package server wires the HTTP API.
package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/an4eetos/serendipity/api/internal/auth"
	"github.com/an4eetos/serendipity/api/internal/llm"
	"github.com/an4eetos/serendipity/api/internal/openlibrary"
	"github.com/an4eetos/serendipity/api/internal/storage"
	"github.com/an4eetos/serendipity/api/internal/store"
)

type Server struct {
	store       *store.Store
	auth        *auth.Verifier
	llm         *llm.Client
	openLibrary *openlibrary.Client
	storage     *storage.Client
}

func New(st *store.Store, v *auth.Verifier, l *llm.Client, ol *openlibrary.Client, sc *storage.Client) *Server {
	return &Server{store: st, auth: v, llm: l, openLibrary: ol, storage: sc}
}

func (s *Server) Routes(allowedOrigins []string) http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.RequestID, middleware.RealIP, middleware.Logger, middleware.Recoverer)
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins: allowedOrigins,
		AllowedMethods: []string{"GET", "POST", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders: []string{"Authorization", "Content-Type"},
		MaxAge:         300,
	}))

	r.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) { w.Write([]byte("ok")) })

	r.With(s.auth.Optional).Get("/profiles/{username}", s.getPublicProfile)

	r.Group(func(r chi.Router) {
		r.Use(s.auth.Require)

		r.Get("/me", s.getMe)
		r.Patch("/me", s.patchMe)

		r.With(middleware.Timeout(15*time.Second)).Get("/works/search", s.searchWorks)

		r.Get("/library", s.listLibrary)
		r.Post("/library", s.addToLibrary)
		r.Get("/library/{id}", s.getLibraryItem)
		r.Patch("/library/{id}", s.patchLibraryItem)
		r.Get("/library/{id}/file-url", s.getFileURL)

		r.Post("/imports/kindle", s.importKindle)

		r.Get("/quotes", s.listQuotes)
		r.Post("/quotes", s.createQuote)
		r.Get("/quotes/{id}", s.getQuote)
		r.Patch("/quotes/{id}", s.patchQuote)
		r.Delete("/quotes/{id}", s.deleteQuote)

		r.Get("/personas", s.listPersonas)

		r.Post("/conversations", s.createConversation)
		r.Post("/conversations/{id}/messages", s.postMessage)
	})
	return r
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

type apiError struct {
	status int
	msg    string
}

func (e apiError) Error() string { return e.msg }

func badRequest(format string, args ...any) error {
	return apiError{http.StatusBadRequest, fmt.Sprintf(format, args...)}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, r *http.Request, err error) {
	var ae apiError
	var pgErr *pgconn.PgError
	switch {
	case errors.As(err, &ae):
		writeJSON(w, ae.status, map[string]string{"error": ae.msg})
	case errors.Is(err, store.ErrNotFound),
		// A malformed id in the URL can't match anything.
		errors.As(err, &pgErr) && pgErr.Code == "22P02":
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
	default:
		// A client that went away is not a server error.
		if r.Context().Err() == nil {
			slog.Error("request failed", "path", r.URL.Path, "err", err)
		}
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
	}
}

func decodeJSON(r *http.Request, v any) error {
	dec := json.NewDecoder(http.MaxBytesReader(nil, r.Body, 1<<20))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return badRequest("invalid JSON body: %v", err)
	}
	return nil
}

// sse writes server-sent events.
type sse struct {
	w http.ResponseWriter
	f http.Flusher
}

func newSSE(w http.ResponseWriter) (*sse, error) {
	f, ok := w.(http.Flusher)
	if !ok {
		return nil, errors.New("streaming unsupported")
	}
	h := w.Header()
	h.Set("Content-Type", "text/event-stream")
	h.Set("Cache-Control", "no-cache")
	h.Set("Connection", "keep-alive")
	h.Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	f.Flush()
	return &sse{w: w, f: f}, nil
}

func (s *sse) send(v any) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	if _, err := fmt.Fprintf(s.w, "data: %s\n\n", b); err != nil {
		return err
	}
	s.f.Flush()
	return nil
}
