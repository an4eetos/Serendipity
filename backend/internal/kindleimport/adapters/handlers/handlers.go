// Package handlers serves the Kindle import over HTTP.
package handlers

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/an4eetos/serendipity/backend/internal/kernel/adapters/httpapi"
	"github.com/an4eetos/serendipity/backend/internal/kindleimport/adapters/converters"
	"github.com/an4eetos/serendipity/backend/internal/kindleimport/usecase"
)

const maxClippingsUpload = 20 << 20

type Handler struct {
	importer *usecase.ImportClippings
}

func New(importer *usecase.ImportClippings) *Handler {
	return &Handler{importer: importer}
}

func (h *Handler) RegisterRoutes(_, private chi.Router) {
	private.Post("/imports/kindle", h.importKindle)
}

func (h *Handler) importKindle(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxClippingsUpload)
	file, _, err := r.FormFile("file")
	if err != nil {
		httpapi.WriteError(w, r, httpapi.BadRequest(`upload your My Clippings.txt as the "file" field`))
		return
	}
	defer file.Close()
	confirmed := r.FormValue("ownership_confirmed") == "true"
	report, err := h.importer.Execute(r.Context(), httpapi.UserID(r.Context()), file, confirmed)
	if err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, converters.ToReportResponse(report))
}
