package domain

import (
	"errors"
	"reflect"
	"strings"
	"testing"
)

func ptr[T any](v T) *T { return &v }

func TestNewWork(t *testing.T) {
	tests := []struct {
		name    string
		in      Work
		want    Work
		wantErr error
	}{
		{
			name: "trims the title",
			in:   Work{Title: "  Meditations  "},
			want: Work{Title: "Meditations"},
		},
		{"blank title", Work{Title: "   "}, Work{}, ErrTitleRequired},
		{"title too long", Work{Title: strings.Repeat("a", MaxTitleLength+1)}, Work{}, ErrTitleRequired},
		{
			name: "title at the cap",
			in:   Work{Title: strings.Repeat("a", MaxTitleLength)},
			want: Work{Title: strings.Repeat("a", MaxTitleLength)},
		},
		{"foreign Open Library key", Work{Title: "x", OpenLibraryKey: ptr("/books/OL1M")}, Work{}, ErrInvalidWorkKey},
		{
			name: "keeps a work key and trusted cover",
			in:   Work{Title: "x", OpenLibraryKey: ptr("/works/OL1W"), CoverURL: ptr("https://covers.openlibrary.org/b/id/1-M.jpg")},
			want: Work{Title: "x", OpenLibraryKey: ptr("/works/OL1W"), CoverURL: ptr("https://covers.openlibrary.org/b/id/1-M.jpg")},
		},
		{
			name: "drops an untrusted cover",
			in:   Work{Title: "x", CoverURL: ptr("https://evil.example/cover.jpg")},
			want: Work{Title: "x"},
		},
		{
			name: "caps subjects",
			in:   Work{Title: "x", Subjects: []string{"1", "2", "3", "4", "5", "6", "7", "8", "9", "10"}},
			want: Work{Title: "x", Subjects: []string{"1", "2", "3", "4", "5", "6", "7", "8"}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewWork(tt.in)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v, want %v", err, tt.wantErr)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("NewWork() = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestSearchQuery(t *testing.T) {
	if _, err := SearchQuery("  a "); !errors.Is(err, ErrQueryTooShort) {
		t.Errorf("SearchQuery(short) err = %v", err)
	}
	if q, err := SearchQuery("  plato "); err != nil || q != "plato" {
		t.Errorf("SearchQuery() = %q, %v", q, err)
	}
}
