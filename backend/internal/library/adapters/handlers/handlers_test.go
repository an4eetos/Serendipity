package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/an4eetos/serendipity/backend/internal/kernel/adapters/httpapi"
	"github.com/an4eetos/serendipity/backend/internal/kernel/ports/portstest"
	"github.com/an4eetos/serendipity/backend/internal/library/domain"
	"github.com/an4eetos/serendipity/backend/internal/library/ports"
	"github.com/an4eetos/serendipity/backend/internal/library/usecase"
)

type fakeLibrary struct {
	ports.LibraryRepository
	patches []domain.Patch
}

func (f *fakeLibrary) Update(_ context.Context, _, id string, p domain.Patch) (domain.Item, error) {
	f.patches = append(f.patches, p)
	confirmed := time.Date(2026, 10, 24, 12, 0, 0, 0, time.UTC)
	return domain.Item{ID: id, FilePath: p.FilePath, FileType: p.FileType, OwnershipConfirmedAt: &confirmed}, nil
}

func TestPatchLibraryItemOwnership(t *testing.T) {
	tests := []struct {
		name        string
		body        string
		wantStatus  int
		wantInBody  string
		wantPatches int
	}{
		{"file without the confirmation", `{"file_path":"u1/a.epub","file_type":"epub"}`,
			http.StatusBadRequest, `{"error":"confirm that you own this copy first"}`, 0},
		{"file with the confirmation false", `{"file_path":"u1/a.epub","file_type":"epub","ownership_confirmed":false}`,
			http.StatusBadRequest, `{"error":"confirm that you own this copy first"}`, 0},
		{"file with the confirmation", `{"file_path":"u1/a.epub","file_type":"epub","ownership_confirmed":true}`,
			http.StatusOK, `"ownership_confirmed_at":"2026-10-24T12:00:00Z"`, 1},
		{"progress needs no confirmation", `{"progress":0.5}`, http.StatusOK, `"id":"item-1"`, 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lib := &fakeLibrary{}
			r := chi.NewRouter()
			r.Use(func(next http.Handler) http.Handler {
				return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					next.ServeHTTP(w, r.WithContext(httpapi.WithUserID(r.Context(), "u1")))
				})
			})
			New(nil, nil, nil, usecase.NewUpdateBook(&portstest.TxManager{}, lib), nil).RegisterRoutes(r, r)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, httptest.NewRequest(http.MethodPatch, "/library/item-1", strings.NewReader(tt.body)))
			if w.Code != tt.wantStatus || !strings.Contains(w.Body.String(), tt.wantInBody) || len(lib.patches) != tt.wantPatches {
				t.Errorf("got %d %q, %d patches; want %d containing %q, %d patches",
					w.Code, w.Body.String(), len(lib.patches), tt.wantStatus, tt.wantInBody, tt.wantPatches)
			}
			if tt.wantPatches == 1 && strings.Contains(tt.body, "file_path") && !lib.patches[0].OwnershipConfirmed {
				t.Errorf("stored %#v, want the confirmation passed on", lib.patches[0])
			}
		})
	}
}
