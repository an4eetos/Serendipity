// Package handlers serves moderation over HTTP: readers' reports, and the
// admin routes that review them.
package handlers

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/an4eetos/serendipity/backend/internal/kernel/adapters/httpapi"
	"github.com/an4eetos/serendipity/backend/internal/moderation/adapters/converters"
	"github.com/an4eetos/serendipity/backend/internal/moderation/usecase"
)

// Handler serves readers' routes and admins' routes, so the router mounts
// it on both groups.
var (
	_ httpapi.Routes      = (*Handler)(nil)
	_ httpapi.AdminRoutes = (*Handler)(nil)
)

// Handler serves moderation's HTTP routes.
type Handler struct {
	create    *usecase.CreateReport
	reports   *usecase.ListOpenReports
	hide      *usecase.HideTarget
	dismiss   *usecase.DismissReports
	takedowns *usecase.ListTakedowns
	restore   *usecase.RestoreTarget
}

// New returns the moderation handlers.
func New(
	create *usecase.CreateReport,
	reports *usecase.ListOpenReports,
	hide *usecase.HideTarget,
	dismiss *usecase.DismissReports,
	takedowns *usecase.ListTakedowns,
	restore *usecase.RestoreTarget,
) *Handler {
	return &Handler{
		create:    create,
		reports:   reports,
		hide:      hide,
		dismiss:   dismiss,
		takedowns: takedowns,
		restore:   restore,
	}
}

// RegisterRoutes implements httpapi.Routes.
func (h *Handler) RegisterRoutes(_, private chi.Router) {
	private.Post("/reports", h.createReport)
}

// RegisterAdminRoutes implements httpapi.AdminRoutes.
func (h *Handler) RegisterAdminRoutes(admin chi.Router) {
	admin.Get("/admin/reports", h.listReports)
	admin.Post("/admin/reports/dismiss", h.dismissReports)
	admin.Get("/admin/takedowns", h.listTakedowns)
	admin.Post("/admin/takedowns", h.hideTarget)
	admin.Post("/admin/takedowns/{id}/restore", h.restoreTarget)
}

func (h *Handler) createReport(w http.ResponseWriter, r *http.Request) {
	var req converters.CreateReportRequest
	if err := httpapi.DecodeJSON(r, &req); err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	rep, err := h.create.Execute(r.Context(), httpapi.UserID(r.Context()), req.ToTargetRef(), req.ToReason(), req.Note)
	if err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusCreated, converters.ToReportResponse(rep))
}

func (h *Handler) listReports(w http.ResponseWriter, r *http.Request) {
	ts, err := h.reports.Execute(r.Context())
	if err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, converters.ToReportedTargetResponses(ts))
}

func (h *Handler) dismissReports(w http.ResponseWriter, r *http.Request) {
	var req converters.DismissRequest
	if err := httpapi.DecodeJSON(r, &req); err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	n, err := h.dismiss.Execute(r.Context(), httpapi.UserID(r.Context()), req.ToTargetRef())
	if err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, converters.DismissResponse{Dismissed: n})
}

func (h *Handler) listTakedowns(w http.ResponseWriter, r *http.Request) {
	vs, err := h.takedowns.Execute(r.Context())
	if err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, converters.ToTakedownTargetResponses(vs))
}

func (h *Handler) hideTarget(w http.ResponseWriter, r *http.Request) {
	var req converters.HideRequest
	if err := httpapi.DecodeJSON(r, &req); err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	td, err := h.hide.Execute(r.Context(), httpapi.UserID(r.Context()), req.ToTargetRef(), req.Reason)
	if err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusCreated, converters.ToTakedownResponse(td))
}

// restoreTarget takes no body: an empty one or {} is fine.
func (h *Handler) restoreTarget(w http.ResponseWriter, r *http.Request) {
	td, err := h.restore.Execute(r.Context(), httpapi.UserID(r.Context()), chi.URLParam(r, "id"))
	if err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, converters.ToTakedownResponse(td))
}
