package usecase

import (
	"context"

	discussionports "github.com/an4eetos/serendipity/backend/internal/discussion/ports"
	kernelports "github.com/an4eetos/serendipity/backend/internal/kernel/ports"
	"github.com/an4eetos/serendipity/backend/internal/moderation/domain"
	"github.com/an4eetos/serendipity/backend/internal/moderation/ports"
	quoteports "github.com/an4eetos/serendipity/backend/internal/quote/ports"
)

// HideTarget takes content down from every public read. Its open reports
// are closed as actioned.
type HideTarget struct {
	txm       kernelports.TxManager
	reports   ports.ReportRepository
	takedowns ports.TakedownRepository
	targets   targets
}

// NewHideTarget returns the HideTarget usecase.
func NewHideTarget(
	txm kernelports.TxManager,
	reports ports.ReportRepository,
	takedowns ports.TakedownRepository,
	quotes quoteports.QuoteModeration,
	conversations discussionports.ConversationModeration,
) *HideTarget {
	return &HideTarget{
		txm:       txm,
		reports:   reports,
		takedowns: takedowns,
		targets:   targets{quotes: quotes, conversations: conversations},
	}
}

// Execute hides ref for adminID, telling the owner reason.
func (uc *HideTarget) Execute(ctx context.Context, adminID string, ref domain.TargetRef, reason string) (domain.Takedown, error) {
	if err := ref.Validate(); err != nil {
		return domain.Takedown{}, err
	}
	t, err := uc.targets.load(ctx, ref)
	if err != nil {
		return domain.Takedown{}, err
	}
	td, err := domain.NewTakedown(t, reason, adminID)
	if err != nil {
		return domain.Takedown{}, err
	}

	var created domain.Takedown
	err = uc.txm.WithinTx(ctx, func(ctx context.Context) error {
		if err := uc.targets.remove(ctx, td.Target, td.Reason); err != nil {
			return err
		}
		if created, err = uc.takedowns.Create(ctx, td); err != nil {
			return err
		}
		_, err = uc.reports.Resolve(ctx, td.Target, domain.StatusActioned, adminID)
		return err
	})
	return created, err
}
