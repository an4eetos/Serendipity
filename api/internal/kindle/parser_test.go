package kindle

import (
	"os"
	"reflect"
	"strings"
	"testing"
)

func TestParse(t *testing.T) {
	raw, err := os.ReadFile("testdata/My Clippings.txt")
	if err != nil {
		t.Fatal(err)
	}

	// Real files come with a byte order mark (sometimes before every title)
	// and CRLF line endings; all variants must parse identically.
	variants := map[string]string{
		"plain": string(raw),
		"bom":   "\ufeff" + string(raw),
		"crlf":  strings.ReplaceAll(string(raw), "\n", "\r\n"),
		"bom-per-entry-crlf": strings.ReplaceAll(
			strings.ReplaceAll(string(raw), "==========\n", "==========\n\ufeff"), "\n", "\r\n"),
	}

	want := []Book{
		{
			Title:  "The Republic",
			Author: "Plato",
			Highlights: []Highlight{
				{Text: "The beginning is the most important part of the work.", Page: "12", LocStart: 170, LocEnd: 172},
			},
			OrphanNotes: 1,
		},
		{
			Title:  "Meditations (Penguin Classics)",
			Author: "Marcus Aurelius",
			Highlights: []Highlight{
				{
					Text:     "You have power over your mind - not outside events. Realize this, and you will find strength.",
					Note:     "Easy to say on a quiet evening.",
					LocStart: 88,
					LocEnd:   91,
				},
			},
		},
		{
			Title:  "Untitled Notes",
			Author: "",
			Highlights: []Highlight{
				{Text: "A personal document\nwith two lines.", Page: "3"},
			},
		},
	}

	for name, input := range variants {
		t.Run(name, func(t *testing.T) {
			got, err := Parse(strings.NewReader(input))
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, want) {
				t.Errorf("Parse() mismatch\n got: %#v\nwant: %#v", got, want)
			}
		})
	}
}

func TestSplitTitleAuthor(t *testing.T) {
	tests := []struct{ in, title, author string }{
		{"The Republic (Plato)", "The Republic", "Plato"},
		{"Meditations (Penguin Classics) (Marcus Aurelius)", "Meditations (Penguin Classics)", "Marcus Aurelius"},
		{"Thinking, Fast and Slow (Kahneman, Daniel)", "Thinking, Fast and Slow", "Kahneman, Daniel"},
		{"No Author Here", "No Author Here", ""},
		{"(Only Parens)", "(Only Parens)", ""},
	}
	for _, tt := range tests {
		title, author := splitTitleAuthor(tt.in)
		if title != tt.title || author != tt.author {
			t.Errorf("splitTitleAuthor(%q) = %q, %q; want %q, %q", tt.in, title, author, tt.title, tt.author)
		}
	}
}

func TestHighlightLocation(t *testing.T) {
	tests := []struct {
		h    Highlight
		want string
	}{
		{Highlight{Page: "12", LocStart: 170, LocEnd: 172}, "Page 12 · Loc 170-172"},
		{Highlight{LocStart: 88, LocEnd: 88}, "Loc 88"},
		{Highlight{Page: "iv"}, "Page iv"},
		{Highlight{}, ""},
	}
	for _, tt := range tests {
		if got := tt.h.Location(); got != tt.want {
			t.Errorf("Location() = %q, want %q", got, tt.want)
		}
	}
}
