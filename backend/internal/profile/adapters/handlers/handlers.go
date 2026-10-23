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
	terms    *usecase.GetTerms
	accept   *usecase.AcceptTerms
}

var (
	_ httpapi.Routes      = (*Handler)(nil)
	_ httpapi.TermsRoutes = (*Handler)(nil)
)

func New(me *usecase.GetMe, updateMe *usecase.UpdateMe, public *usecase.GetPublicProfile,
	terms *usecase.GetTerms, accept *usecase.AcceptTerms) *Handler {
	return &Handler{me: me, updateMe: updateMe, public: public, terms: terms, accept: accept}
}

func (h *Handler) RegisterRoutes(public, private chi.Router) {
	public.Get("/terms", h.getTerms)
	public.Get("/profiles/{username}", h.getPublicProfile)
	private.Get("/me", h.getMe)
	private.Patch("/me", h.patchMe)
}

// RegisterTermsRoutes mounts accepting the terms, which readers must reach
// before they have accepted them.
func (h *Handler) RegisterTermsRoutes(signedIn chi.Router) {
	signedIn.Post("/me/terms", h.acceptTerms)
}

func (h *Handler) getTerms(w http.ResponseWriter, r *http.Request) {
	httpapi.WriteJSON(w, http.StatusOK, converters.ToTermsResponse(h.terms.Execute(r.Context())))
}

func (h *Handler) acceptTerms(w http.ResponseWriter, r *http.Request) {
	var req converters.AcceptTermsRequest
	if err := httpapi.DecodeJSON(r, &req); err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	me, err := h.accept.Execute(r.Context(), httpapi.UserID(r.Context()), req.Version)
	if err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, converters.ToMeResponse(me))
}

func (h *Handler) getMe(w http.ResponseWriter, r *http.Request) {
	me, err := h.me.Execute(r.Context(), httpapi.UserID(r.Context()))
	if err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, converters.ToMeResponse(me))
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
