package httpapi

import (
	"errors"
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
