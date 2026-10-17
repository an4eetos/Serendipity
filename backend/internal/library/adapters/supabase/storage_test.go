package supabase

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestSignedURL(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.EscapedPath() != "/storage/v1/object/sign/books/user-1/My%20Book.epub" {
			t.Errorf("unexpected request %s %s", r.Method, r.URL.EscapedPath())
		}
		if r.Header.Get("Authorization") != "Bearer service-key" || r.Header.Get("apikey") != "service-key" {
			t.Error("missing service key")
		}
		var body map[string]int
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body["expiresIn"] != 600 {
			t.Errorf("body = %v, %v", body, err)
		}
		w.Write([]byte(`{"signedURL":"/object/sign/books/user-1/My%20Book.epub?token=t"}`))
	}))
	defer srv.Close()

	got, err := NewStorage(srv.URL, "service-key").SignedURL(t.Context(), "user-1/My Book.epub", 10*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if want := srv.URL + "/storage/v1/object/sign/books/user-1/My%20Book.epub?token=t"; got != want {
		t.Errorf("SignedURL() = %q, want %q", got, want)
	}
}

func TestSignedURLFailure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	if _, err := NewStorage(srv.URL, "k").SignedURL(t.Context(), "u/a.pdf", time.Minute); err == nil {
		t.Fatal("want an error")
	}
}
