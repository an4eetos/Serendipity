// Package ports declares what the moderation context needs from the outside.
// It acts on readers' content through the quote and discussion contexts'
// moderation ports.
package ports

import (
	"context"

	"github.com/an4eetos/serendipity/backend/internal/moderation/domain"
)

// ReportRepository stores readers' reports.
type ReportRepository interface {
	// Create stores an open report. It returns domain.ErrAlreadyReported when
	// the reporter already has an open report on the target, and
	// domain.ErrOwnContent when the reporter owns it.
	Create(ctx context.Context, r domain.Report) (domain.Report, error)
	// ListOpen lists up to limit open reports, oldest first, with the target
	// owner's username.
	ListOpen(ctx context.Context, limit int) ([]domain.Report, error)
	// Resolve closes every open report on target with status, and returns
	// how many it closed.
	Resolve(ctx context.Context, target domain.TargetRef, status domain.ReportStatus, adminID string) (int, error)
}

// TakedownRepository stores takedowns.
type TakedownRepository interface {
	// Create stores an active takedown. It returns domain.ErrAlreadyRemoved
	// when the target already has one.
	Create(ctx context.Context, t domain.Takedown) (domain.Takedown, error)
	Get(ctx context.Context, id string) (domain.Takedown, error)
	// ListActive lists up to limit active takedowns, newest first.
	ListActive(ctx context.Context, limit int) ([]domain.Takedown, error)
	// MarkRestored ends the active takedown id. It returns
	// domain.ErrAlreadyRestored when it is no longer active.
	MarkRestored(ctx context.Context, id, adminID string) (domain.Takedown, error)
}

// AdminRepository knows who the admins are.
type AdminRepository interface {
	IsAdmin(ctx context.Context, userID string) (bool, error)
}
