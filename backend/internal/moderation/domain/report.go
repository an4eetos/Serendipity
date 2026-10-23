// Package domain holds the moderation context's rules: what readers may
// report, and how admins hide, dismiss and restore reported content.
package domain

import (
	"strings"
	"time"
	"unicode/utf8"

	kernel "github.com/an4eetos/serendipity/backend/internal/kernel/domain"
)

const (
	// MaxNoteLength caps a reporter's note, in characters.
	MaxNoteLength = 1000
	// MaxTakedownReasonLength caps the reason an admin gives the owner, in
	// characters.
	MaxTakedownReasonLength = 500
	// MaxListed caps each admin list (open reports, active takedowns).
	MaxListed = 200
)

// Errors of reports.
var (
	ErrInvalidTarget   = kernel.Invalid("you can report a quote or a discussion")
	ErrInvalidReason   = kernel.Invalid("pick a reason: copyright, harassment, hate, spam or other")
	ErrNoteTooLong     = kernel.Invalid("the note is longer than 1000 characters")
	ErrAlreadyReported = kernel.Conflict("you already reported this")
)

// TargetKind is the kind of thing a report or takedown is about.
type TargetKind string

const (
	// TargetQuote is a reader's quote.
	TargetQuote TargetKind = "quote"
	// TargetConversation is a discussion of a quote with a character.
	TargetConversation TargetKind = "conversation"
)

// Valid reports whether k is a kind readers can report.
func (k TargetKind) Valid() bool {
	return k == TargetQuote || k == TargetConversation
}

// TargetRef names one reportable thing.
type TargetRef struct {
	Kind TargetKind
	ID   string
}

// Validate reports whether r names a reportable kind and an id.
func (r TargetRef) Validate() error {
	if !r.Kind.Valid() || r.ID == "" {
		return ErrInvalidTarget
	}
	return nil
}

// Reason is why a reader reports something.
type Reason string

const (
	ReasonCopyright  Reason = "copyright"
	ReasonHarassment Reason = "harassment"
	ReasonHate       Reason = "hate"
	ReasonSpam       Reason = "spam"
	ReasonOther      Reason = "other"
)

// Valid reports whether r is one of the reasons readers can pick.
func (r Reason) Valid() bool {
	switch r {
	case ReasonCopyright, ReasonHarassment, ReasonHate, ReasonSpam, ReasonOther:
		return true
	default:
		return false
	}
}

// ReportStatus is where a report stands.
type ReportStatus string

const (
	// StatusOpen means no admin has looked at the report yet.
	StatusOpen ReportStatus = "open"
	// StatusActioned means an admin hid the target.
	StatusActioned ReportStatus = "actioned"
	// StatusDismissed means an admin left the target up, or restored it.
	StatusDismissed ReportStatus = "dismissed"
)

// Report is a reader's request that admins look at someone else's content.
type Report struct {
	ID         string
	ReporterID string
	Target     TargetRef
	OwnerID    string
	// OwnerUsername is read only: it comes from the owner's profile.
	OwnerUsername string
	Reason        Reason
	Note          string
	Status        ReportStatus
	CreatedAt     time.Time
}

// NewReport checks a reader's report. The note is trimmed. The owner is set
// once the target has been looked up.
func NewReport(reporterID string, target TargetRef, reason Reason, note string) (Report, error) {
	if err := target.Validate(); err != nil {
		return Report{}, err
	}
	if !reason.Valid() {
		return Report{}, ErrInvalidReason
	}
	note = strings.TrimSpace(note)
	if utf8.RuneCountInString(note) > MaxNoteLength {
		return Report{}, ErrNoteTooLong
	}
	return Report{
		ReporterID: reporterID,
		Target:     target,
		Reason:     reason,
		Note:       note,
		Status:     StatusOpen,
	}, nil
}
