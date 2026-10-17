// Package middleware authenticates HTTP requests.
package middleware

import (
	"net/http"
	"strings"

	"github.com/an4eetos/serendipity/backend/internal/identity/ports"
	"github.com/an4eetos/serendipity/backend/internal/kernel/adapters/httpapi"
)

type Auth struct {
	verifier ports.TokenVerifier
}

func NewAuth(v ports.TokenVerifier) *Auth {
	return &Auth{verifier: v}
}

// Require rejects requests without a valid access token.
func (a *Auth) Require(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		uid, err := a.verifier.Verify(bearer(r))
		if err != nil {
			http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r.WithContext(httpapi.WithUserID(r.Context(), uid)))
	})
}

// Optional attaches the user id when a valid token is present.
func (a *Auth) Optional(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if t := bearer(r); t != "" {
			if uid, err := a.verifier.Verify(t); err == nil {
				r = r.WithContext(httpapi.WithUserID(r.Context(), uid))
			}
		}
		next.ServeHTTP(w, r)
	})
}

func bearer(r *http.Request) string {
	h := r.Header.Get("Authorization")
	if t, ok := strings.CutPrefix(h, "Bearer "); ok {
		return strings.TrimSpace(t)
	}
	return ""
}
