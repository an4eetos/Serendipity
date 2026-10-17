// Package handlers serves the catalog over HTTP.
package handlers

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/an4eetos/serendipity/backend/internal/catalog/adapters/converters"
	"github.com/an4eetos/serendipity/backend/internal/catalog/usecase"
	"github.com/an4eetos/serendipity/backend/internal/kernel/adapters/httpapi"
)

type Handler struct {
	search *usecase.SearchWorks
}

func New(search *usecase.SearchWorks) *Handler {
	return &Handler{search: search}
}

func (h *Handler) RegisterRoutes(_, private chi.Router) {
	private.With(middleware.Timeout(15*time.Second)).Get("/works/search", h.searchWorks)
}

func (h *Handler) searchWorks(w http.ResponseWriter, r *http.Request) {
	results, err := h.search.Execute(r.Context(), r.URL.Query().Get("q"))
	if err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, converters.ToSearchResultResponses(results))
}
