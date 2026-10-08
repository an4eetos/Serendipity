package quote

import (
	"errors"
	"strings"
	"testing"
)

func TestValidateText(t *testing.T) {
	tests := []struct {
		name string
		text string
		want error
	}{
		{"empty", "   \n\t", ErrEmpty},
		{"one word", "Hello", nil},
		{"at cap", strings.Repeat("word ", MaxWords), nil},
		{"over cap", strings.Repeat("word ", MaxWords+1), ErrTooLong},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ValidateText(tt.text); !errors.Is(got, tt.want) {
				t.Errorf("ValidateText() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestValidatePublic(t *testing.T) {
	tests := []struct {
		name                 string
		text, note, location string
		want                 error
	}{
		{"ok", "To be or not to be", "My take", "Act 3", nil},
		{"no note", "To be or not to be", "  ", "Act 3", ErrNoteRequired},
		{"no location", "To be or not to be", "My take", "", ErrCitationMissing},
		{"too long", strings.Repeat("w ", MaxWords+1), "My take", "Act 3", ErrTooLong},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ValidatePublic(tt.text, tt.note, tt.location); !errors.Is(got, tt.want) {
				t.Errorf("ValidatePublic() = %v, want %v", got, tt.want)
			}
		})
	}
}
