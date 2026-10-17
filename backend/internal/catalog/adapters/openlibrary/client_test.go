package openlibrary

import (
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/an4eetos/serendipity/backend/internal/catalog/domain"
)

func TestSearch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/search.json" || r.URL.Query().Get("q") != "plato" {
			t.Errorf("unexpected request %s", r.URL)
		}
		if ua := r.Header.Get("User-Agent"); ua == "" {
			t.Error("missing User-Agent")
		}
		w.Write([]byte(`{"docs":[
			{"key":"/works/OL1W","title":"Republic","author_name":["Plato"],"isbn":["111","222"],"cover_i":42,"subject":["Ethics"],"first_publish_year":-375},
			{"key":"","title":"No key"},
			{"key":"/works/OL2W","title":"Apology"}
		]}`))
	}))
	defer srv.Close()

	got, err := New(srv.URL).Search(t.Context(), "plato")
	if err != nil {
		t.Fatal(err)
	}
	isbn, cover, year := "111", "https://covers.openlibrary.org/b/id/42-M.jpg", -375
	want := []domain.SearchResult{
		{OpenLibraryKey: "/works/OL1W", Title: "Republic", Authors: []string{"Plato"}, ISBN: &isbn, CoverURL: &cover, Subjects: []string{"Ethics"}, FirstPublished: &year},
		{OpenLibraryKey: "/works/OL2W", Title: "Apology"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Search() = %#v\nwant %#v", got, want)
	}
}

func TestSearchFailure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	if _, err := New(srv.URL).Search(t.Context(), "plato"); err == nil {
		t.Fatal("want an error for a failed search")
	}
}
