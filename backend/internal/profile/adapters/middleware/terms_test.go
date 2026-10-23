package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/an4eetos/serendipity/backend/internal/kernel/adapters/httpapi"
	kernel "github.com/an4eetos/serendipity/backend/internal/kernel/domain"
	"github.com/an4eetos/serendipity/backend/internal/profile/domain"
	"github.com/an4eetos/serendipity/backend/internal/profile/ports"
	"github.com/an4eetos/serendipity/backend/internal/profile/usecase"
)

type fakeProfiles struct {
	ports.ProfileRepository
	byID    map[string]domain.Profile
	lookups int
}

func (f *fakeProfiles) Get(_ context.Context, id string) (domain.Profile, error) {
	f.lookups++
	p, ok := f.byID[id]
	if !ok {
		return domain.Profile{}, kernel.ErrNotFound
	}
	return p, nil
}

func TestTermsGate(t *testing.T) {
	accepted := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	profiles := &fakeProfiles{byID: map[string]domain.Profile{
		"new":      {ID: "new"},
		"outdated": {ID: "outdated", TermsVersion: "v1", TermsAcceptedAt: &accepted},
		"current":  {ID: "current", TermsVersion: "v2", TermsAcceptedAt: &accepted},
	}}
	gate := NewTerms(usecase.NewCheckTerms(profiles, domain.Terms{Version: "v2"})).Gate
	const refused = `{"error":"accept the current terms of service first"}` + "\n"

	tests := []struct {
		name        string
		method      string
		reader      string
		wantStatus  int
		wantBody    string
		wantLookups int
	}{
		{"reads pass without the terms", http.MethodGet, "new", http.StatusOK, "next", 0},
		{"HEAD is a read", http.MethodHead, "new", http.StatusOK, "next", 0},
		{"OPTIONS is a read", http.MethodOptions, "new", http.StatusOK, "next", 0},
		{"POST before accepting", http.MethodPost, "new", http.StatusForbidden, refused, 1},
		{"PATCH before accepting", http.MethodPatch, "new", http.StatusForbidden, refused, 1},
		{"DELETE before accepting", http.MethodDelete, "new", http.StatusForbidden, refused, 1},
		{"PUT before accepting", http.MethodPut, "new", http.StatusForbidden, refused, 1},
		{"POST after accepting an older version", http.MethodPost, "outdated", http.StatusForbidden, refused, 1},
		{"POST after accepting the current version", http.MethodPost, "current", http.StatusOK, "next", 1},
		{"DELETE after accepting the current version", http.MethodDelete, "current", http.StatusOK, "next", 1},
		{"no profile", http.MethodPost, "missing", http.StatusNotFound, `{"error":"not found"}` + "\n", 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			profiles.lookups = 0
			next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("next")) })
			r := httptest.NewRequest(tt.method, "/quotes", nil)
			r = r.WithContext(httpapi.WithUserID(r.Context(), tt.reader))
			w := httptest.NewRecorder()
			gate(next).ServeHTTP(w, r)
			if w.Code != tt.wantStatus || w.Body.String() != tt.wantBody || profiles.lookups != tt.wantLookups {
				t.Errorf("got %d %q with %d lookups, want %d %q with %d",
					w.Code, w.Body.String(), profiles.lookups, tt.wantStatus, tt.wantBody, tt.wantLookups)
			}
		})
	}
}
