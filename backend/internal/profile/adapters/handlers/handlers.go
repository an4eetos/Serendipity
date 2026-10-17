// Package handlers serves readers' profiles over HTTP.
package handlers

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/an4eetos/serendipity/backend/internal/kernel/adapters/httpapi"
	"github.com/an4eetos/serendipity/backend/internal/profile/adapters/converters"
	"github.com/an4eetos/serendipity/backend/internal/profile/usecase"
)

type Handler struct {
	me       *usecase.GetMe
	updateMe *usecase.UpdateMe
	public   *usecase.GetPublicProfile
}

func New(me *usecase.GetMe, updateMe *usecase.UpdateMe, public *usecase.GetPublicProfile) *Handler {
	return &Handler{me: me, updateMe: updateMe, public: public}
}

func (h *Handler) RegisterRoutes(public, private chi.Router) {
	public.Get("/profiles/{username}", h.getPublicProfile)
	private.Get("/me", h.getMe)
	private.Patch("/me", h.patchMe)
}

func (h *Handler) getMe(w http.ResponseWriter, r *http.Request) {
	p, err := h.me.Execute(r.Context(), httpapi.UserID(r.Context()))
	if err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, converters.ToProfileResponse(p))
}

func (h *Handler) patchMe(w http.ResponseWriter, r *http.Request) {
	var req converters.PatchRequest
	if err := httpapi.DecodeJSON(r, &req); err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	p, err := h.updateMe.Execute(r.Context(), httpapi.UserID(r.Context()), req.ToPatch())
	if err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, converters.ToProfileResponse(p))
}

func (h *Handler) getPublicProfile(w http.ResponseWriter, r *http.Request) {
	p, err := h.public.Execute(r.Context(), httpapi.UserID(r.Context()), chi.URLParam(r, "username"))
	if err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, converters.ToPublicProfileResponse(p))
}
