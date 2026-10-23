package middleware

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/an4eetos/serendipity/backend/internal/kernel/adapters/httpapi"
	"github.com/an4eetos/serendipity/backend/internal/moderation/usecase"
)

type fakeAdmins struct {
	admins map[string]bool
	err    error
}

func (f fakeAdmins) IsAdmin(_ context.Context, userID string) (bool, error) {
	return f.admins[userID], f.err
}

func TestAdminRequire(t *testing.T) {
	admins := fakeAdmins{admins: map[string]bool{"root": true}}
	const refused = `{"error":"admins only"}` + "\n"
	tests := []struct {
		name       string
		admins     fakeAdmins
		reader     string
		method     string
		wantStatus int
		wantBody   string
	}{
		{"admin reads", admins, "root", http.MethodGet, http.StatusOK, "next"},
		{"admin writes", admins, "root", http.MethodPost, http.StatusOK, "next"},
		{"reader reads", admins, "bo", http.MethodGet, http.StatusForbidden, refused},
		{"reader writes", admins, "bo", http.MethodPost, http.StatusForbidden, refused},
		{"no reader in the context", admins, "", http.MethodGet, http.StatusForbidden, refused},
		{"lookup fails", fakeAdmins{err: errors.New("connection reset")}, "root", http.MethodGet,
			http.StatusInternalServerError, `{"error":"internal error"}` + "\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("next")) })
			r := httptest.NewRequest(tt.method, "/admin/reports", nil)
			if tt.reader != "" {
				r = r.WithContext(httpapi.WithUserID(r.Context(), tt.reader))
			}
			w := httptest.NewRecorder()
			NewAdmin(usecase.NewCheckAdmin(tt.admins)).Require(next).ServeHTTP(w, r)
			if w.Code != tt.wantStatus || w.Body.String() != tt.wantBody {
				t.Errorf("got %d %q, want %d %q", w.Code, w.Body.String(), tt.wantStatus, tt.wantBody)
			}
		})
	}
}
