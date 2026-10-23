package domain

import (
	kernel "github.com/an4eetos/serendipity/backend/internal/kernel/domain"
)

// Errors of admins' review.
var (
	ErrNotAdmin      = kernel.Forbidden("admins only")
	ErrNoOpenReports = kernel.NotFound("no open reports on this")
)

// ReportedTarget is one target's open reports, as admins review them.
type ReportedTarget struct {
	Ref           TargetRef
	OwnerID       string
	OwnerUsername string
	// Content is nil when the content was deleted.
	Content *Target
	Reports []Report
}

// GroupReports groups reports by target, keeping the order of each target's
// first report and, within a target, the order of its reports.
func GroupReports(reports []Report) []ReportedTarget {
	out := []ReportedTarget{}
	index := make(map[TargetRef]int, len(reports))
	for _, r := range reports {
		i, ok := index[r.Target]
		if !ok {
			i = len(out)
			index[r.Target] = i
			out = append(out, ReportedTarget{
				Ref:           r.Target,
				OwnerID:       r.OwnerID,
				OwnerUsername: r.OwnerUsername,
			})
		}
		out[i].Reports = append(out[i].Reports, r)
	}
	return out
}

// TakedownView is an active takedown with the content it hides.
type TakedownView struct {
	Takedown Takedown
	// Content is nil when the content was deleted.
	Content *Target
}
