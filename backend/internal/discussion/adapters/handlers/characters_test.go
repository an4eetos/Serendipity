package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/an4eetos/serendipity/backend/internal/discussion/domain"
	"github.com/an4eetos/serendipity/backend/internal/discussion/ports"
	"github.com/an4eetos/serendipity/backend/internal/discussion/usecase"
	"github.com/an4eetos/serendipity/backend/internal/kernel/adapters/httpapi"
	kernel "github.com/an4eetos/serendipity/backend/internal/kernel/domain"
	"github.com/an4eetos/serendipity/backend/internal/kernel/ports/portstest"
)

var created = time.Date(2026, 11, 16, 12, 0, 0, 0, time.UTC)

// fakeCharacters implements ports.CharacterRepository for reader "u1".
type fakeCharacters struct {
	ports.CharacterRepository
	byID   map[string]domain.Persona
	active int
}

func (f *fakeCharacters) ListOwn(context.Context, string) ([]domain.Persona, error) {
	return []domain.Persona{f.byID["used"]}, nil
}

func (f *fakeCharacters) GetOwn(_ context.Context, ownerID, id string) (domain.Persona, error) {
	p, ok := f.byID[id]
	if !ok || p.OwnerID != ownerID {
		return domain.Persona{}, kernel.ErrNotFound
	}
	return p, nil
}

func (f *fakeCharacters) GetOwnForUpdate(ctx context.Context, ownerID, id string) (domain.Persona, error) {
	return f.GetOwn(ctx, ownerID, id)
}

func (f *fakeCharacters) LockOwner(context.Context, string) error { return nil }

func (f *fakeCharacters) CountActive(context.Context, string) (int, error) { return f.active, nil }

func (f *fakeCharacters) Create(_ context.Context, ownerID string, d domain.CharacterDraft, slugBase string) (domain.Persona, error) {
	return domain.Persona{ID: "new", Slug: slugBase + "-0123456789", Name: d.Name, Era: d.Era, ShortBio: d.ShortBio,
		SystemPrompt: d.Prompt, OwnerID: ownerID, CreatedAt: created, UpdatedAt: created}, nil
}

func (f *fakeCharacters) Update(_ context.Context, p domain.Persona) (domain.Persona, error) {
	return p, nil
}

func serveCharacters(t *testing.T, active int, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	removedAt := created
	chars := &fakeCharacters{active: active, byID: map[string]domain.Persona{
		"used":     {ID: "used", Slug: "ada-0123456789", Name: "Ada", SystemPrompt: "You are Ada.", OwnerID: "u1", HasConversations: true},
		"archived": {ID: "archived", Name: "Bo", SystemPrompt: "x", OwnerID: "u1", ArchivedAt: &created},
		"removed":  {ID: "removed", Name: "Cy", SystemPrompt: "x", OwnerID: "u1", RemovedAt: &removedAt, RemovedReason: "spam"},
		"theirs":   {ID: "theirs", Name: "Di", SystemPrompt: "x", OwnerID: "u2"},
	}}
	txm := &portstest.TxManager{}
	const limit domain.CharacterLimit = 2
	h := NewCharacters(usecase.NewListCharacters(chars, limit), usecase.NewCreateCharacter(txm, chars, limit),
		usecase.NewUpdateCharacter(txm, chars), usecase.NewArchiveCharacter(txm, chars),
		usecase.NewUnarchiveCharacter(txm, chars, limit))
	r := chi.NewRouter()
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			next.ServeHTTP(w, r.WithContext(httpapi.WithUserID(r.Context(), "u1")))
		})
	})
	public := chi.NewRouter()
	h.RegisterRoutes(public, r)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(method, path, strings.NewReader(body)))
	return w
}

func TestCharactersRoutes(t *testing.T) {
	const ada = `{"name":"  Ada  ","era":"1840s","short_bio":"Countess.","prompt":"You are Ada."}`
	tests := []struct {
		name         string
		active       int
		method, path string
		body         string
		wantStatus   int
		wantBody     []string
	}{
		{"list", 0, http.MethodGet, "/characters", "", http.StatusOK,
			[]string{`"active":1`, `"active_limit":2`, `"prompt":"You are Ada."`, `"name_locked":true`, `"status":"active"`}},
		{"create", 1, http.MethodPost, "/characters", ada, http.StatusCreated,
			[]string{`"id":"new"`, `"slug":"ada-0123456789"`, `"name":"Ada"`, `"prompt":"You are Ada."`, `"status":"active"`, `"name_locked":false`, `"archived_at":null`}},
		{"create at the cap", 2, http.MethodPost, "/characters", ada, http.StatusConflict,
			[]string{`{"error":"you've reached your limit of active characters — archive one first"}`}},
		{"create with a blank prompt", 0, http.MethodPost, "/characters", `{"name":"Ada","prompt":"  "}`, http.StatusBadRequest,
			[]string{`{"error":"prompt must be 1–8000 characters"}`}},
		{"create with an unknown field", 0, http.MethodPost, "/characters", `{"name":"Ada","system_prompt":"x"}`, http.StatusBadRequest,
			[]string{`invalid JSON body`}},
		{"rename a used character", 0, http.MethodPatch, "/characters/used", `{"name":"Augusta"}`, http.StatusConflict,
			[]string{`{"error":"the name can't change once the character has been in a discussion"}`}},
		{"edit a used character's bio", 0, http.MethodPatch, "/characters/used", `{"short_bio":"Mathematician."}`, http.StatusOK,
			[]string{`"short_bio":"Mathematician."`, `"name":"Ada"`}},
		{"edit someone else's", 0, http.MethodPatch, "/characters/theirs", `{"short_bio":"x"}`, http.StatusNotFound, nil},
		{"edit a removed one", 0, http.MethodPatch, "/characters/removed", `{"short_bio":"x"}`, http.StatusConflict,
			[]string{`{"error":"a moderator removed this character"}`}},
		{"archive", 0, http.MethodPost, "/characters/used/archive", "", http.StatusOK, []string{`"status":"archived"`}},
		{"archive a removed one", 0, http.MethodPost, "/characters/removed/archive", "", http.StatusConflict, nil},
		{"unarchive", 1, http.MethodPost, "/characters/archived/unarchive", "", http.StatusOK, []string{`"status":"active"`}},
		{"unarchive at the cap", 2, http.MethodPost, "/characters/archived/unarchive", "", http.StatusConflict,
			[]string{`active characters`}},
		{"unarchive someone else's", 0, http.MethodPost, "/characters/theirs/unarchive", "", http.StatusNotFound, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := serveCharacters(t, tt.active, tt.method, tt.path, tt.body)
			if w.Code != tt.wantStatus {
				t.Fatalf("status %d %q, want %d", w.Code, w.Body.String(), tt.wantStatus)
			}
			for _, want := range tt.wantBody {
				if !strings.Contains(w.Body.String(), want) {
					t.Errorf("body %q lacks %q", w.Body.String(), want)
				}
			}
		})
	}
}
