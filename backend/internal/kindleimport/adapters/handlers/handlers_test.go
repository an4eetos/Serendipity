package handlers

import (
	"bytes"
	"context"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"

	catalog "github.com/an4eetos/serendipity/backend/internal/catalog/domain"
	"github.com/an4eetos/serendipity/backend/internal/kernel/adapters/httpapi"
	kernel "github.com/an4eetos/serendipity/backend/internal/kernel/domain"
	"github.com/an4eetos/serendipity/backend/internal/kernel/ports/portstest"
	"github.com/an4eetos/serendipity/backend/internal/kindleimport/usecase"
	library "github.com/an4eetos/serendipity/backend/internal/library/domain"
	libraryports "github.com/an4eetos/serendipity/backend/internal/library/ports"
	quote "github.com/an4eetos/serendipity/backend/internal/quote/domain"
	quoteports "github.com/an4eetos/serendipity/backend/internal/quote/ports"
)

type fakeWorks struct{}

func (fakeWorks) Ensure(_ context.Context, w catalog.Work, _ string) (catalog.Work, error) {
	w.ID = "work-1"
	return w, nil
}

type fakeLibrary struct {
	libraryports.LibraryRepository
	confirmed []string
}

func (*fakeLibrary) FindByTitle(context.Context, string, string) (library.Item, error) {
	return library.Item{}, kernel.ErrNotFound
}

func (*fakeLibrary) Add(_ context.Context, _, workID string) (library.Item, error) {
	return library.Item{ID: "item-1", Work: catalog.Work{ID: workID}}, nil
}

func (f *fakeLibrary) Update(_ context.Context, _, id string, p library.Patch) (library.Item, error) {
	if p.OwnershipConfirmed {
		f.confirmed = append(f.confirmed, id)
	}
	return library.Item{ID: id}, nil
}

type fakeQuotes struct{ quoteports.QuoteRepository }

func (fakeQuotes) Import(context.Context, string, quote.Draft) (bool, error) { return true, nil }

const clippings = "The Republic (Plato)\n- Your Highlight on page 12 | Location 170-172\n\nThe beginning is the most important part of the work.\n==========\n"

func upload(t *testing.T, fields map[string]string) *http.Request {
	t.Helper()
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	fw, err := mw.CreateFormFile("file", "My Clippings.txt")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fw.Write([]byte(clippings)); err != nil {
		t.Fatal(err)
	}
	for k, v := range fields {
		if err := mw.WriteField(k, v); err != nil {
			t.Fatal(err)
		}
	}
	if err := mw.Close(); err != nil {
		t.Fatal(err)
	}
	r := httptest.NewRequest(http.MethodPost, "/imports/kindle", &body)
	r.Header.Set("Content-Type", mw.FormDataContentType())
	return r.WithContext(httpapi.WithUserID(r.Context(), "u1"))
}

func TestImportKindleOwnership(t *testing.T) {
	const refused = `{"error":"confirm that you own this copy first"}` + "\n"
	tests := []struct {
		name       string
		fields     map[string]string
		wantStatus int
		wantBody   string
	}{
		{"without the confirmation", nil, http.StatusBadRequest, refused},
		{"confirmation not true", map[string]string{"ownership_confirmed": "yes"}, http.StatusBadRequest, refused},
		{"with the confirmation", map[string]string{"ownership_confirmed": "true"}, http.StatusOK,
			`{"books":1,"imported":1,"duplicates":0,"skipped_too_long":0,"orphan_notes":0}` + "\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lib := &fakeLibrary{}
			r := chi.NewRouter()
			New(usecase.NewImportClippings(&portstest.TxManager{}, fakeWorks{}, lib, fakeQuotes{})).RegisterRoutes(r, r)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, upload(t, tt.fields))
			if w.Code != tt.wantStatus || w.Body.String() != tt.wantBody {
				t.Errorf("got %d %q, want %d %q", w.Code, w.Body.String(), tt.wantStatus, tt.wantBody)
			}
			if wantConfirmed := tt.wantStatus == http.StatusOK; (len(lib.confirmed) == 1) != wantConfirmed {
				t.Errorf("confirmed items %v", lib.confirmed)
			}
		})
	}
}
