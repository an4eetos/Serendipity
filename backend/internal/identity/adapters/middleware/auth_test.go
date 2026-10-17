package middleware

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/an4eetos/serendipity/backend/internal/kernel/adapters/httpapi"
)

type fakeVerifier map[string]string

func (f fakeVerifier) Verify(token string) (string, error) {
	if uid, ok := f[token]; ok {
		return uid, nil
	}
	return "", errors.New("bad token")
}

func TestAuth(t *testing.T) {
	auth := NewAuth(fakeVerifier{"good": "user-1"})
	var seen string
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = httpapi.UserID(r.Context())
	})

	tests := []struct {
		name       string
		mw         func(http.Handler) http.Handler
		header     string
		wantStatus int
		wantUser   string
	}{
		{"require valid", auth.Require, "Bearer good", http.StatusOK, "user-1"},
		{"require invalid", auth.Require, "Bearer bad", http.StatusUnauthorized, ""},
		{"require missing", auth.Require, "", http.StatusUnauthorized, ""},
		{"optional valid", auth.Optional, "Bearer good", http.StatusOK, "user-1"},
		{"optional invalid stays anonymous", auth.Optional, "Bearer bad", http.StatusOK, ""},
		{"optional missing", auth.Optional, "", http.StatusOK, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			seen = ""
			r := httptest.NewRequest(http.MethodGet, "/", nil)
			if tt.header != "" {
				r.Header.Set("Authorization", tt.header)
			}
			w := httptest.NewRecorder()
			tt.mw(next).ServeHTTP(w, r)
			if w.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", w.Code, tt.wantStatus)
			}
			if seen != tt.wantUser {
				t.Errorf("user = %q, want %q", seen, tt.wantUser)
			}
		})
	}
}
