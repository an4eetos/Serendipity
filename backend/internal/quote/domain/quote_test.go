package domain

import (
	"errors"
	"reflect"
	"strings"
	"testing"
)

func ptr[T any](v T) *T { return &v }

func TestNewUpload(t *testing.T) {
	tests := []struct {
		name    string
		in      Draft
		want    Draft
		wantErr error
	}{
		{
			name: "trims and marks the source",
			in:   Draft{Text: "  To be or not to be  ", Location: "  Act 3  ", Reaction: "Hm", Position: ptr(0.4)},
			want: Draft{Text: "To be or not to be", Location: "Act 3", Reaction: "Hm", Position: ptr(0.4), Source: SourceUpload},
		},
		{"empty text", Draft{Text: "   "}, Draft{}, ErrEmpty},
		{"too many words", Draft{Text: strings.Repeat("w ", MaxWords+1)}, Draft{}, ErrTooLong},
		{"reaction too long", Draft{Text: "x", Reaction: strings.Repeat("a", MaxReactionLength+1)}, Draft{}, ErrReactionTooLong},
		{
			name: "drops a position outside the book",
			in:   Draft{Text: "x", Position: ptr(1.5)},
			want: Draft{Text: "x", Source: SourceUpload},
		},
		{
			name: "cuts the citation by characters",
			in:   Draft{Text: "x", Location: strings.Repeat("é", MaxLocationLength+10)},
			want: Draft{Text: "x", Location: strings.Repeat("é", MaxLocationLength), Source: SourceUpload},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewUpload(tt.in)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v, want %v", err, tt.wantErr)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("NewUpload() = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestCheckPatch(t *testing.T) {
	cited := Quote{Text: "To be", Location: "Act 3", Reaction: "Mine", Visibility: VisibilityPrivate}
	uncited := Quote{Text: "To be", Reaction: "Mine", Visibility: VisibilityPrivate}
	public := Quote{Text: "To be", Location: "Act 3", Reaction: "Mine", Visibility: VisibilityPublic}
	tests := []struct {
		name  string
		quote Quote
		patch Patch
		want  error
	}{
		{"publish", cited, Patch{Visibility: ptr(VisibilityPublic)}, nil},
		{"publish without citation", uncited, Patch{Visibility: ptr(VisibilityPublic)}, ErrCitationMissing},
		{"publish while clearing the reaction", cited, Patch{Visibility: ptr(VisibilityPublic), Reaction: ptr(" ")}, ErrReactionRequired},
		{"clear the reaction of a public quote", public, Patch{Reaction: ptr("")}, ErrReactionRequired},
		{"unpublish and clear the reaction", public, Patch{Visibility: ptr(VisibilityPrivate), Reaction: ptr("")}, nil},
		{"unknown visibility", cited, Patch{Visibility: ptr(Visibility("friends"))}, ErrInvalidVisibility},
		{"reaction too long", cited, Patch{Reaction: ptr(strings.Repeat("a", MaxReactionLength+1))}, ErrReactionTooLong},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.quote.CheckPatch(tt.patch); !errors.Is(got, tt.want) {
				t.Errorf("CheckPatch() = %v, want %v", got, tt.want)
			}
		})
	}
}
