// Package middleware guards HTTP routes with the profile context's rules.
package middleware

import (
	"net/http"

	"github.com/an4eetos/serendipity/backend/internal/kernel/adapters/httpapi"
	"github.com/an4eetos/serendipity/backend/internal/profile/usecase"
)

// Terms holds back changes from readers who haven't accepted the current
// terms of service.
type Terms struct {
	check *usecase.CheckTerms
}

func NewTerms(check *usecase.CheckTerms) *Terms {
	return &Terms{check: check}
}

// Gate refuses anything but GET, HEAD and OPTIONS from readers who haven't
// accepted the current terms, with httpapi.WriteError (403). It must run
// after the reader is authenticated.
func (t *Terms) Gate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if readOnly(r.Method) {
			next.ServeHTTP(w, r)
			return
		}
		if err := t.check.Execute(r.Context(), httpapi.UserID(r.Context())); err != nil {
			httpapi.WriteError(w, r, err)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// readOnly reports whether a request with this method changes nothing.
func readOnly(method string) bool {
	switch method {
	case http.MethodGet, http.MethodHead, http.MethodOptions:
		return true
	default:
		return false
	}
}
