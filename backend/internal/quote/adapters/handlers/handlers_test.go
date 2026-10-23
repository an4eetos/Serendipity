package handlers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
)

// The reader can't decide whether a public passage is shown: the request
// has no such field, and decoding refuses it before any usecase runs.
func TestPatchQuoteRefusesPassageHidden(t *testing.T) {
	r := chi.NewRouter()
	New(nil, nil, nil, nil).RegisterRoutes(r, r)
	for _, body := range []string{`{"passage_hidden":false}`, `{"visibility":"public","passage_hidden":false}`} {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(http.MethodPatch, "/quotes/q1", strings.NewReader(body)))
		want := `{"error":"invalid JSON body: json: unknown field \"passage_hidden\""}` + "\n"
		if w.Code != http.StatusBadRequest || w.Body.String() != want {
			t.Errorf("%s: got %d %q, want 400 %q", body, w.Code, w.Body.String(), want)
		}
	}
}
