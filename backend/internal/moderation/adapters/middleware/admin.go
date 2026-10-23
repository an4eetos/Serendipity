// Package middleware lets only admins through to the admin routes.
package middleware

import (
	"net/http"

	"github.com/an4eetos/serendipity/backend/internal/kernel/adapters/httpapi"
	"github.com/an4eetos/serendipity/backend/internal/moderation/usecase"
)

// Admin guards the admin routes.
type Admin struct {
	check *usecase.CheckAdmin
}

// NewAdmin returns middleware that asks check whether the reader is an admin.
func NewAdmin(check *usecase.CheckAdmin) *Admin {
	return &Admin{check: check}
}

// Require refuses signed-in readers who aren't admins (403). It must run
// after the identity context's Auth.Require, which sets the reader's id.
func (a *Admin) Require(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := a.check.Execute(r.Context(), httpapi.UserID(r.Context())); err != nil {
			httpapi.WriteError(w, r, err)
			return
		}
		next.ServeHTTP(w, r)
	})
}
