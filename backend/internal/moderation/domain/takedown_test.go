package domain

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func TestNewTakedown(t *testing.T) {
	ref := TargetRef{Kind: TargetQuote, ID: "q1"}
	up := Target{Ref: ref, OwnerID: "owner", Public: true}
	gone := Target{Ref: ref, OwnerID: "owner", Removed: true}
	tests := []struct {
		name       string
		target     Target
		reason     string
		wantErr    error
		wantReason string
	}{
		{"hides with a reason", up, "Copyright notice from the publisher", nil, "Copyright notice from the publisher"},
		{"trims the reason", up, "  spam  ", nil, "spam"},
		{"one character is enough", up, "x", nil, "x"},
		{"reason at the limit, counted in characters", up, strings.Repeat("é", MaxTakedownReasonLength), nil, strings.Repeat("é", MaxTakedownReasonLength)},
		{"no reason", up, "", ErrTakedownReason, ""},
		{"only spaces", up, " \t\n ", ErrTakedownReason, ""},
		{"reason too long", up, strings.Repeat("a", MaxTakedownReasonLength+1), ErrTakedownReason, ""},
		{"already hidden", gone, "again", ErrAlreadyRemoved, ""},
		{"a private target can still be hidden", Target{Ref: ref, OwnerID: "owner"}, "x", nil, "x"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			td, err := NewTakedown(tt.target, tt.reason, "admin-1")
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v, want %v", err, tt.wantErr)
			}
			if err != nil {
				return
			}
			if td.Target != ref || td.OwnerID != "owner" || td.Reason != tt.wantReason ||
				td.HiddenBy == nil || *td.HiddenBy != "admin-1" || td.RestoredAt != nil || td.RestoredBy != nil {
				t.Errorf("NewTakedown() = %#v", td)
			}
		})
	}
}

func TestCheckRestorable(t *testing.T) {
	if err := (Takedown{}).CheckRestorable(); err != nil {
		t.Errorf("active takedown: %v", err)
	}
	restored := time.Date(2026, 10, 24, 0, 0, 0, 0, time.UTC)
	if err := (Takedown{RestoredAt: &restored}).CheckRestorable(); !errors.Is(err, ErrAlreadyRestored) {
		t.Errorf("restored takedown: err = %v, want ErrAlreadyRestored", err)
	}
}
