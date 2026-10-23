package domain

import (
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"
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
	removedAt := time.Date(2026, 10, 24, 0, 0, 0, 0, time.UTC)
	removed := cited
	removed.RemovedAt, removed.RemovedReason = &removedAt, "copyright"
	removedUncited := uncited
	removedUncited.RemovedAt = &removedAt
	removedPublic := public
	removedPublic.RemovedAt = &removedAt
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
		{"publish a removed quote", removed, Patch{Visibility: ptr(VisibilityPublic)}, ErrRemoved},
		{"removal is checked before the publishing rules", removedUncited, Patch{Visibility: ptr(VisibilityPublic)}, ErrRemoved},
		{"edit the reaction of a removed quote", removed, Patch{Reaction: ptr("Still mine")}, nil},
		{"keep a removed quote private", removed, Patch{Visibility: ptr(VisibilityPrivate)}, nil},
		{"unpublish a removed public quote", removedPublic, Patch{Visibility: ptr(VisibilityPrivate)}, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.quote.CheckPatch(tt.patch); !errors.Is(got, tt.want) {
				t.Errorf("CheckPatch() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestQuoteVisibility(t *testing.T) {
	removedAt := time.Date(2026, 10, 24, 0, 0, 0, 0, time.UTC)
	tests := []struct {
		name                  string
		quote                 Quote
		wantRemoved, wantSeen bool
	}{
		{"private", Quote{Visibility: VisibilityPrivate}, false, false},
		{"public", Quote{Visibility: VisibilityPublic}, false, true},
		{"public but removed", Quote{Visibility: VisibilityPublic, RemovedAt: &removedAt}, true, false},
		{"private and removed", Quote{Visibility: VisibilityPrivate, RemovedAt: &removedAt}, true, false},
		{"public with a collapsed passage", Quote{Visibility: VisibilityPublic, PassageHidden: true}, false, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.quote.Removed() != tt.wantRemoved || tt.quote.IsPublic() != tt.wantSeen {
				t.Errorf("Removed() = %v, IsPublic() = %v; want %v, %v",
					tt.quote.Removed(), tt.quote.IsPublic(), tt.wantRemoved, tt.wantSeen)
			}
		})
	}
}

func TestPublishes(t *testing.T) {
	private, public := Quote{Visibility: VisibilityPrivate}, Quote{Visibility: VisibilityPublic}
	tests := []struct {
		name  string
		quote Quote
		patch Patch
		want  bool
	}{
		{"publish a private quote", private, Patch{Visibility: ptr(VisibilityPublic)}, true},
		{"publish a public quote again", public, Patch{Visibility: ptr(VisibilityPublic)}, false},
		{"unpublish", public, Patch{Visibility: ptr(VisibilityPrivate)}, false},
		{"edit the reaction only", private, Patch{Reaction: ptr("x")}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.quote.Publishes(tt.patch); got != tt.want {
				t.Errorf("Publishes() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestForPublic(t *testing.T) {
	removedAt := time.Date(2026, 10, 24, 0, 0, 0, 0, time.UTC)
	shown := Quote{ID: "q1", Text: "To be", Reaction: "Mine", Location: "Act 3", Visibility: VisibilityPublic,
		RemovedAt: &removedAt, RemovedReason: "copyright"}
	got := shown.ForPublic()
	if got.Text != "To be" || got.Reaction != "Mine" || got.Location != "Act 3" {
		t.Errorf("shown passage: ForPublic() = %#v, want text, reaction and citation kept", got)
	}
	if got.RemovedAt != nil || got.RemovedReason != "" {
		t.Errorf("ForPublic() keeps the removal details %v %q; only the owner sees them", got.RemovedAt, got.RemovedReason)
	}

	collapsed := shown
	collapsed.PassageHidden = true
	got = collapsed.ForPublic()
	if got.Text != "" || !got.PassageHidden || got.Reaction != "Mine" || got.Location != "Act 3" {
		t.Errorf("collapsed passage: ForPublic() = %#v, want the text blanked and the reaction and citation kept", got)
	}
	if collapsed.Text != "To be" || collapsed.RemovedReason != "copyright" {
		t.Error("ForPublic() changed the quote it was called on")
	}
}
