package httpserver

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/an4eetos/serendipity/backend/infra/config"
	cataloghandlers "github.com/an4eetos/serendipity/backend/internal/catalog/adapters/handlers"
	discussionhandlers "github.com/an4eetos/serendipity/backend/internal/discussion/adapters/handlers"
	identitymw "github.com/an4eetos/serendipity/backend/internal/identity/adapters/middleware"
	"github.com/an4eetos/serendipity/backend/internal/kernel/adapters/httpapi"
	kernel "github.com/an4eetos/serendipity/backend/internal/kernel/domain"
	"github.com/an4eetos/serendipity/backend/internal/kernel/ports/portstest"
	kindlehandlers "github.com/an4eetos/serendipity/backend/internal/kindleimport/adapters/handlers"
	libraryhandlers "github.com/an4eetos/serendipity/backend/internal/library/adapters/handlers"
	moderationhandlers "github.com/an4eetos/serendipity/backend/internal/moderation/adapters/handlers"
	moderationmw "github.com/an4eetos/serendipity/backend/internal/moderation/adapters/middleware"
	moderation "github.com/an4eetos/serendipity/backend/internal/moderation/usecase"
	profilehandlers "github.com/an4eetos/serendipity/backend/internal/profile/adapters/handlers"
	profilemw "github.com/an4eetos/serendipity/backend/internal/profile/adapters/middleware"
	profile "github.com/an4eetos/serendipity/backend/internal/profile/domain"
	profileports "github.com/an4eetos/serendipity/backend/internal/profile/ports"
	profileusecase "github.com/an4eetos/serendipity/backend/internal/profile/usecase"
	quotehandlers "github.com/an4eetos/serendipity/backend/internal/quote/adapters/handlers"
)

// tokens maps the bearer tokens of the test readers to their ids.
var tokens = map[string]string{
	"token-new":     "new",     // hasn't accepted the terms
	"token-current": "current", // accepted the current terms, not an admin
	"token-admin":   "root",    // an admin who hasn't accepted the reader terms
}

type fakeVerifier struct{}

func (fakeVerifier) Verify(token string) (string, error) {
	if uid, ok := tokens[token]; ok {
		return uid, nil
	}
	return "", errors.New("bad token")
}

type fakeProfiles struct {
	profileports.ProfileRepository
	byID map[string]profile.Profile
}

func (f *fakeProfiles) Get(_ context.Context, id string) (profile.Profile, error) {
	p, ok := f.byID[id]
	if !ok {
		return profile.Profile{}, kernel.ErrNotFound
	}
	return p, nil
}

func (f *fakeProfiles) AcceptTerms(_ context.Context, id, version string) (profile.Profile, error) {
	p, ok := f.byID[id]
	if !ok {
		return profile.Profile{}, kernel.ErrNotFound
	}
	now := time.Now()
	p.TermsVersion, p.TermsAcceptedAt = version, &now
	f.byID[id] = p
	return p, nil
}

type fakeAdmins map[string]bool

func (f fakeAdmins) IsAdmin(_ context.Context, userID string) (bool, error) { return f[userID], nil }

// probe has a write on the private group and one on the admin group that
// succeed when a request reaches them.
type probe struct{}

func (probe) RegisterRoutes(_, private chi.Router) {
	private.Post("/probe", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
}

func (probe) RegisterAdminRoutes(admin chi.Router) {
	admin.Post("/admin/probe", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
}

const currentTerms = "v2"

// newTestRouter mounts every bounded context's real handlers. Their
// usecases are nil, apart from the profile's, so a request that gets past
// the middleware to one of them panics and comes back as a 500.
func newTestRouter() http.Handler {
	accepted := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	profiles := &fakeProfiles{byID: map[string]profile.Profile{
		"new":     {ID: "new", Username: "new_reader"},
		"current": {ID: "current", Username: "current_reader", TermsVersion: currentTerms, TermsAcceptedAt: &accepted},
		"root":    {ID: "root", Username: "root"},
	}}
	terms := profile.Terms{Version: currentTerms}
	profileRoutes := profilehandlers.New(
		profileusecase.NewGetMe(profiles, terms),
		profileusecase.NewUpdateMe(&portstest.TxManager{}, profiles),
		nil,
		profileusecase.NewGetTerms(terms),
		profileusecase.NewAcceptTerms(&portstest.TxManager{}, profiles, terms),
	)
	return NewRouter(RouterParams{
		Config: config.Config{HTTP: config.HTTP{AllowedOrigins: []string{"http://localhost:3000"}}},
		Auth:   identitymw.NewAuth(fakeVerifier{}),
		Terms:  profilemw.NewTerms(profileusecase.NewCheckTerms(profiles, terms)),
		Admin:  moderationmw.NewAdmin(moderation.NewCheckAdmin(fakeAdmins{"root": true})),
		Routes: []httpapi.Routes{
			cataloghandlers.New(nil),
			libraryhandlers.New(nil, nil, nil, nil, nil),
			quotehandlers.New(nil, nil, nil, nil),
			discussionhandlers.New(nil, nil, nil, nil),
			kindlehandlers.New(nil),
			profileRoutes,
			moderationhandlers.New(nil, nil, nil, nil, nil, nil),
			probe{},
		},
	})
}

type route struct{ method, pattern string }

func routes(t *testing.T, h http.Handler) []route {
	t.Helper()
	var out []route
	err := chi.Walk(h.(chi.Routes), func(method, pattern string, _ http.Handler, _ ...func(http.Handler) http.Handler) error {
		out = append(out, route{method, pattern})
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func send(h http.Handler, method, path, token, body string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, path, strings.NewReader(body))
	if token != "" {
		r.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}

// path fills a route pattern's parameters.
func path(pattern string) string {
	r := strings.NewReplacer("{id}", "00000000-0000-4000-8000-000000000001", "{username}", "ada")
	return r.Replace(pattern)
}

func readOnly(method string) bool {
	return method == http.MethodGet || method == http.MethodHead || method == http.MethodOptions
}

func TestRouterGatesEveryWriteOnTheTerms(t *testing.T) {
	h := newTestRouter()
	const refused = `{"error":"accept the current terms of service first"}` + "\n"
	var gated []route
	for _, rt := range routes(t, h) {
		if readOnly(rt.method) || rt.pattern == "/me/terms" || strings.HasPrefix(rt.pattern, "/admin/") {
			continue
		}
		gated = append(gated, rt)
		t.Run(rt.method+" "+rt.pattern, func(t *testing.T) {
			if w := send(h, rt.method, path(rt.pattern), "token-new", `{}`); w.Code != http.StatusForbidden || w.Body.String() != refused {
				t.Errorf("before accepting the terms: %d %q, want 403 %q", w.Code, w.Body.String(), refused)
			}
			if w := send(h, rt.method, path(rt.pattern), "", `{}`); w.Code != http.StatusUnauthorized {
				t.Errorf("signed out: %d, want 401", w.Code)
			}
		})
	}
	// Every write that existed before Phase 1, plus reports, is behind the gate.
	for _, want := range []route{
		{"PATCH", "/me"}, {"POST", "/library"}, {"PATCH", "/library/{id}"}, {"POST", "/imports/kindle"},
		{"POST", "/quotes"}, {"PATCH", "/quotes/{id}"}, {"DELETE", "/quotes/{id}"},
		{"POST", "/conversations"}, {"POST", "/conversations/{id}/messages"}, {"POST", "/reports"},
	} {
		if !slices.Contains(gated, want) {
			t.Errorf("%s %s is not mounted behind the terms gate", want.method, want.pattern)
		}
	}
}

func TestRouterAdminRoutes(t *testing.T) {
	h := newTestRouter()
	const refused = `{"error":"admins only"}` + "\n"
	var admin []route
	for _, rt := range routes(t, h) {
		if !strings.HasPrefix(rt.pattern, "/admin/") {
			continue
		}
		admin = append(admin, rt)
		t.Run(rt.method+" "+rt.pattern, func(t *testing.T) {
			for _, token := range []string{"token-current", "token-new"} {
				if w := send(h, rt.method, path(rt.pattern), token, `{}`); w.Code != http.StatusForbidden || w.Body.String() != refused {
					t.Errorf("%s: %d %q, want 403 %q", token, w.Code, w.Body.String(), refused)
				}
			}
			if w := send(h, rt.method, path(rt.pattern), "", `{}`); w.Code != http.StatusUnauthorized {
				t.Errorf("signed out: %d, want 401", w.Code)
			}
		})
	}
	want := []route{
		{"GET", "/admin/reports"}, {"POST", "/admin/reports/dismiss"}, {"GET", "/admin/takedowns"},
		{"POST", "/admin/takedowns"}, {"POST", "/admin/takedowns/{id}/restore"}, {"POST", "/admin/probe"},
	}
	for _, w := range want {
		if !slices.Contains(admin, w) {
			t.Errorf("%s %s is not mounted behind the admin check", w.method, w.pattern)
		}
	}
	if len(admin) != len(want) {
		t.Errorf("admin routes = %v, want %v", admin, want)
	}

	// Admins act as operators, so the reader terms don't hold them back.
	if w := send(h, http.MethodPost, "/admin/probe", "token-admin", ""); w.Code != http.StatusNoContent {
		t.Errorf("admin who hasn't accepted the reader terms: %d, want 204", w.Code)
	}
}

func TestRouterTermsAcceptance(t *testing.T) {
	h := newTestRouter()

	if w := send(h, http.MethodGet, "/terms", "", ""); w.Code != http.StatusOK || w.Body.String() != `{"version":"v2"}`+"\n" {
		t.Errorf("GET /terms signed out: %d %q", w.Code, w.Body.String())
	}
	if w := send(h, http.MethodGet, "/me", "token-new", ""); w.Code != http.StatusOK ||
		!strings.Contains(w.Body.String(), `"current_terms_version":"v2","terms_accepted":false`) {
		t.Errorf("GET /me before accepting: %d %q; reads must pass the gate", w.Code, w.Body.String())
	}
	if w := send(h, http.MethodPost, "/probe", "token-new", ""); w.Code != http.StatusForbidden {
		t.Errorf("write before accepting: %d, want 403", w.Code)
	}
	if w := send(h, http.MethodPost, "/me/terms", "token-new", `{"version":"v1"}`); w.Code != http.StatusConflict {
		t.Errorf("accepting an old version: %d %q, want 409", w.Code, w.Body.String())
	}
	if w := send(h, http.MethodPost, "/me/terms", "", `{"version":"v2"}`); w.Code != http.StatusUnauthorized {
		t.Errorf("accepting signed out: %d, want 401", w.Code)
	}
	w := send(h, http.MethodPost, "/me/terms", "token-new", `{"version":"v2"}`)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"terms_version":"v2"`) ||
		!strings.Contains(w.Body.String(), `"terms_accepted":true`) {
		t.Fatalf("POST /me/terms before accepting: %d %q; it must be reachable", w.Code, w.Body.String())
	}
	if w := send(h, http.MethodPost, "/probe", "token-new", ""); w.Code != http.StatusNoContent {
		t.Errorf("write after accepting: %d, want 204", w.Code)
	}
	if w := send(h, http.MethodPost, "/probe", "token-current", ""); w.Code != http.StatusNoContent {
		t.Errorf("write by a reader who accepted earlier: %d, want 204", w.Code)
	}
}
