// Package handlers serves the reader's library over HTTP.
package handlers

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/an4eetos/serendipity/backend/internal/kernel/adapters/httpapi"
	"github.com/an4eetos/serendipity/backend/internal/library/adapters/converters"
	"github.com/an4eetos/serendipity/backend/internal/library/usecase"
)

type Handler struct {
	add      *usecase.AddBook
	list     *usecase.ListBooks
	get      *usecase.GetBook
	update   *usecase.UpdateBook
	fileLink *usecase.GetFileLink
}

func New(add *usecase.AddBook, list *usecase.ListBooks, get *usecase.GetBook, update *usecase.UpdateBook, fileLink *usecase.GetFileLink) *Handler {
	return &Handler{add: add, list: list, get: get, update: update, fileLink: fileLink}
}

func (h *Handler) RegisterRoutes(_, private chi.Router) {
	private.Get("/library", h.listLibrary)
	private.Post("/library", h.addToLibrary)
	private.Get("/library/{id}", h.getLibraryItem)
	private.Patch("/library/{id}", h.patchLibraryItem)
	private.Get("/library/{id}/file-url", h.getFileURL)
}

func (h *Handler) addToLibrary(w http.ResponseWriter, r *http.Request) {
	var req converters.AddBookRequest
	if err := httpapi.DecodeJSON(r, &req); err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	item, err := h.add.Execute(r.Context(), httpapi.UserID(r.Context()), req.ToWork())
	if err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusCreated, converters.ToItemResponse(item))
}

func (h *Handler) listLibrary(w http.ResponseWriter, r *http.Request) {
	items, err := h.list.Execute(r.Context(), httpapi.UserID(r.Context()))
	if err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, converters.ToItemResponses(items))
}

func (h *Handler) getLibraryItem(w http.ResponseWriter, r *http.Request) {
	item, err := h.get.Execute(r.Context(), httpapi.UserID(r.Context()), chi.URLParam(r, "id"))
	if err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, converters.ToItemResponse(item))
}

func (h *Handler) patchLibraryItem(w http.ResponseWriter, r *http.Request) {
	var req converters.PatchRequest
	if err := httpapi.DecodeJSON(r, &req); err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	item, err := h.update.Execute(r.Context(), httpapi.UserID(r.Context()), chi.URLParam(r, "id"), req.ToPatch())
	if err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, converters.ToItemResponse(item))
}

func (h *Handler) getFileURL(w http.ResponseWriter, r *http.Request) {
	link, err := h.fileLink.Execute(r.Context(), httpapi.UserID(r.Context()), chi.URLParam(r, "id"))
	if err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, converters.ToFileLinkResponse(link))
}
