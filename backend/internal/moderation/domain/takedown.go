package domain

import (
	"strings"
	"time"
	"unicode/utf8"

	kernel "github.com/an4eetos/serendipity/backend/internal/kernel/domain"
)

// Errors of takedowns.
var (
	ErrTakedownReason  = kernel.Invalid("tell the owner why it was hidden (1–500 characters)")
	ErrAlreadyRemoved  = kernel.Conflict("this is already hidden")
	ErrAlreadyRestored = kernel.Conflict("this was already restored")
)

// Takedown records that an admin hid content from every public read. It is
// active until an admin restores the content.
type Takedown struct {
	ID      string
	Target  TargetRef
	OwnerID string
	// OwnerUsername is read only: it comes from the owner's profile.
	OwnerUsername string
	// Reason is shown to the owner.
	Reason     string
	HiddenBy   *string
	RestoredBy *string
	CreatedAt  time.Time
	RestoredAt *time.Time
}

// NewTakedown checks an admin's decision to hide t. The reason is trimmed.
func NewTakedown(t Target, reason, adminID string) (Takedown, error) {
	reason = strings.TrimSpace(reason)
	if n := utf8.RuneCountInString(reason); n == 0 || n > MaxTakedownReasonLength {
		return Takedown{}, ErrTakedownReason
	}
	if t.Removed {
		return Takedown{}, ErrAlreadyRemoved
	}
	return Takedown{
		Target:   t.Ref,
		OwnerID:  t.OwnerID,
		Reason:   reason,
		HiddenBy: &adminID,
	}, nil
}

// CheckRestorable reports whether the target may be restored: only an active
// takedown can be undone.
func (t Takedown) CheckRestorable() error {
	if t.RestoredAt != nil {
		return ErrAlreadyRestored
	}
	return nil
}
