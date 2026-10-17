// Package handlers serves readers' quotes over HTTP. A quote's page, with
// its discussions, is served by the discussion context.
package handlers

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/an4eetos/serendipity/backend/internal/kernel/adapters/httpapi"
	"github.com/an4eetos/serendipity/backend/internal/quote/adapters/converters"
	"github.com/an4eetos/serendipity/backend/internal/quote/usecase"
)

type Handler struct {
	create *usecase.CreateQuote
	list   *usecase.ListQuotes
	update *usecase.UpdateQuote
	delete *usecase.DeleteQuote
}

func New(create *usecase.CreateQuote, list *usecase.ListQuotes, update *usecase.UpdateQuote, del *usecase.DeleteQuote) *Handler {
	return &Handler{create: create, list: list, update: update, delete: del}
}

func (h *Handler) RegisterRoutes(_, private chi.Router) {
	private.Get("/quotes", h.listQuotes)
	private.Post("/quotes", h.createQuote)
	private.Patch("/quotes/{id}", h.patchQuote)
	private.Delete("/quotes/{id}", h.deleteQuote)
}

func (h *Handler) createQuote(w http.ResponseWriter, r *http.Request) {
	var req converters.CreateRequest
	if err := httpapi.DecodeJSON(r, &req); err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	q, err := h.create.Execute(r.Context(), httpapi.UserID(r.Context()), req.LibraryItemID, req.ToDraft())
	if err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusCreated, converters.ToQuoteResponse(q))
}

func (h *Handler) listQuotes(w http.ResponseWriter, r *http.Request) {
	qs, err := h.list.Execute(r.Context(), httpapi.UserID(r.Context()), r.URL.Query().Get("work_id"))
	if err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, converters.ToQuoteResponses(qs))
}

func (h *Handler) patchQuote(w http.ResponseWriter, r *http.Request) {
	var req converters.PatchRequest
	if err := httpapi.DecodeJSON(r, &req); err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	q, err := h.update.Execute(r.Context(), httpapi.UserID(r.Context()), chi.URLParam(r, "id"), req.ToPatch())
	if err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, converters.ToQuoteResponse(q))
}

func (h *Handler) deleteQuote(w http.ResponseWriter, r *http.Request) {
	if err := h.delete.Execute(r.Context(), httpapi.UserID(r.Context()), chi.URLParam(r, "id")); err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
