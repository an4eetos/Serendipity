package httpapi

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	kernel "github.com/an4eetos/serendipity/backend/internal/kernel/domain"
)

func TestWriteError(t *testing.T) {
	errDown := kernel.Unavailable("search is down")
	tests := []struct {
		name       string
		err        error
		wantStatus int
		wantBody   string
	}{
		{"transport", BadRequest("bad body"), http.StatusBadRequest, `{"error":"bad body"}`},
		{"invalid", kernel.Invalid("too long"), http.StatusBadRequest, `{"error":"too long"}`},
		{"not found", kernel.ErrNotFound, http.StatusNotFound, `{"error":"not found"}`},
		{"conflict", kernel.Conflict("taken"), http.StatusConflict, `{"error":"taken"}`},
		{"unavailable", errDown, http.StatusBadGateway, `{"error":"search is down"}`},
		{"forbidden", kernel.Forbidden("admins only"), http.StatusForbidden, `{"error":"admins only"}`},
		{"wrapped forbidden", fmt.Errorf("gate: %w", kernel.Forbidden("accept the terms")), http.StatusForbidden, `{"error":"accept the terms"}`},
		{"joined domain error keeps its message", errors.Join(errDown, errors.New("dial tcp")), http.StatusBadGateway, `{"error":"search is down"}`},
		{"unexpected", errors.New("boom"), http.StatusInternalServerError, `{"error":"internal error"}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			WriteError(w, httptest.NewRequest(http.MethodGet, "/", nil), tt.err)
			if w.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", w.Code, tt.wantStatus)
			}
			if got := w.Body.String(); got != tt.wantBody+"\n" {
				t.Errorf("body = %q, want %q", got, tt.wantBody+"\n")
			}
			if ct := w.Header().Get("Content-Type"); ct != "application/json" {
				t.Errorf("Content-Type = %q", ct)
			}
		})
	}
}

func TestForbiddenKind(t *testing.T) {
	err := kernel.Forbidden("no")
	if err.Kind() != kernel.KindForbidden || err.Error() != "no" {
		t.Errorf("Forbidden() = kind %v, message %q", err.Kind(), err.Error())
	}
	if got := StatusOf(err); got != http.StatusForbidden {
		t.Errorf("StatusOf(Forbidden) = %d, want 403", got)
	}
	// The new kind didn't shift the existing ones.
	for kind, want := range map[kernel.DomainError]int{
		kernel.Invalid("x"):     http.StatusBadRequest,
		kernel.ErrNotFound:      http.StatusNotFound,
		kernel.Conflict("x"):    http.StatusConflict,
		kernel.Unavailable("x"): http.StatusBadGateway,
	} {
		if got := StatusOf(kind); got != want {
			t.Errorf("StatusOf(%v) = %d, want %d", kind, got, want)
		}
	}
}
