package usecase

import (
	"context"

	discussionports "github.com/an4eetos/serendipity/backend/internal/discussion/ports"
	"github.com/an4eetos/serendipity/backend/internal/moderation/domain"
	"github.com/an4eetos/serendipity/backend/internal/moderation/ports"
	quoteports "github.com/an4eetos/serendipity/backend/internal/quote/ports"
)

// ListOpenReports lists the content readers reported, with its open
// reports, for admins to review.
type ListOpenReports struct {
	reports ports.ReportRepository
	targets targets
}

// NewListOpenReports returns the ListOpenReports usecase.
func NewListOpenReports(
	reports ports.ReportRepository,
	quotes quoteports.QuoteModeration,
	conversations discussionports.ConversationModeration,
) *ListOpenReports {
	return &ListOpenReports{reports: reports, targets: targets{quotes: quotes, conversations: conversations}}
}

// Execute lists the targets of the oldest domain.MaxListed open reports,
// in the order they were first reported.
func (uc *ListOpenReports) Execute(ctx context.Context) ([]domain.ReportedTarget, error) {
	reports, err := uc.reports.ListOpen(ctx, domain.MaxListed)
	if err != nil {
		return nil, err
	}
	groups := domain.GroupReports(reports)
	refs := make([]domain.TargetRef, len(groups))
	for i, g := range groups {
		refs[i] = g.Ref
	}
	content, err := uc.targets.loadAll(ctx, refs)
	if err != nil {
		return nil, err
	}
	for i := range groups {
		if t, ok := content[groups[i].Ref]; ok {
			groups[i].Content = &t
		}
	}
	return groups, nil
}
