package domain

import (
	"errors"
	"strings"
	"testing"
)

func TestNewReport(t *testing.T) {
	quote := TargetRef{Kind: TargetQuote, ID: "q1"}
	tests := []struct {
		name     string
		target   TargetRef
		reason   Reason
		note     string
		wantErr  error
		wantNote string
	}{
		{"quote", quote, ReasonCopyright, "", nil, ""},
		{"conversation", TargetRef{Kind: TargetConversation, ID: "c1"}, ReasonHarassment, "", nil, ""},
		{"trims the note", quote, ReasonOther, "  see page 3  ", nil, "see page 3"},
		{"note at the limit, counted in characters", quote, ReasonSpam, strings.Repeat("é", MaxNoteLength), nil, strings.Repeat("é", MaxNoteLength)},
		{"note too long", quote, ReasonSpam, strings.Repeat("a", MaxNoteLength+1), ErrNoteTooLong, ""},
		{"surrounding spaces don't count", quote, ReasonHate, " " + strings.Repeat("a", MaxNoteLength) + " ", nil, strings.Repeat("a", MaxNoteLength)},
		{"unknown kind", TargetRef{Kind: "comment", ID: "x"}, ReasonSpam, "", ErrInvalidTarget, ""},
		{"no kind", TargetRef{ID: "x"}, ReasonSpam, "", ErrInvalidTarget, ""},
		{"no id", TargetRef{Kind: TargetQuote}, ReasonSpam, "", ErrInvalidTarget, ""},
		{"unknown reason", quote, Reason("boring"), "", ErrInvalidReason, ""},
		{"no reason", quote, "", "", ErrInvalidReason, ""},
		{"the target is checked first", TargetRef{}, "", strings.Repeat("a", MaxNoteLength+1), ErrInvalidTarget, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, err := NewReport("reader-1", tt.target, tt.reason, tt.note)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v, want %v", err, tt.wantErr)
			}
			if err != nil {
				return
			}
			if r.ReporterID != "reader-1" || r.Target != tt.target || r.Reason != tt.reason ||
				r.Note != tt.wantNote || r.Status != StatusOpen || r.OwnerID != "" {
				t.Errorf("NewReport() = %#v", r)
			}
		})
	}
}

func TestKindsAndReasons(t *testing.T) {
	for _, k := range []TargetKind{TargetQuote, TargetConversation} {
		if !k.Valid() {
			t.Errorf("%q must be valid", k)
		}
	}
	for _, k := range []TargetKind{"", "Quote", "comment", "character"} {
		if k.Valid() {
			t.Errorf("%q must not be valid yet", k)
		}
	}
	for _, r := range []Reason{ReasonCopyright, ReasonHarassment, ReasonHate, ReasonSpam, ReasonOther} {
		if !r.Valid() {
			t.Errorf("%q must be valid", r)
		}
	}
	for _, r := range []Reason{"", "Copyright", "abuse"} {
		if r.Valid() {
			t.Errorf("%q must not be valid", r)
		}
	}
}
