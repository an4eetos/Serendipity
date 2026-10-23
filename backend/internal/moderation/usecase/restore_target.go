package usecase

import (
	"context"
	"errors"

	discussionports "github.com/an4eetos/serendipity/backend/internal/discussion/ports"
	kernel "github.com/an4eetos/serendipity/backend/internal/kernel/domain"
	kernelports "github.com/an4eetos/serendipity/backend/internal/kernel/ports"
	"github.com/an4eetos/serendipity/backend/internal/moderation/domain"
	"github.com/an4eetos/serendipity/backend/internal/moderation/ports"
	quote "github.com/an4eetos/serendipity/backend/internal/quote/domain"
	quoteports "github.com/an4eetos/serendipity/backend/internal/quote/ports"
)

// RestoreTarget undoes a takedown: the content is public again, and the
// target's open reports are closed as dismissed. A restored quote counts
// toward its work's coverage again, so it may come back collapsed. When the
// owner has deleted the target, the takedown is closed without restoring.
type RestoreTarget struct {
	txm       kernelports.TxManager
	reports   ports.ReportRepository
	takedowns ports.TakedownRepository
	targets   targets
	wordCap   quote.CoverageCap
}

// NewRestoreTarget returns the RestoreTarget usecase. wordCap is the default
// coverage cap a restored quote is checked against.
func NewRestoreTarget(
	txm kernelports.TxManager,
	reports ports.ReportRepository,
	takedowns ports.TakedownRepository,
	quotes quoteports.QuoteModeration,
	conversations discussionports.ConversationModeration,
	wordCap quote.CoverageCap,
) *RestoreTarget {
	return &RestoreTarget{
		txm:       txm,
		reports:   reports,
		takedowns: takedowns,
		targets:   targets{quotes: quotes, conversations: conversations},
		wordCap:   wordCap,
	}
}

// Execute restores the target of the active takedown takedownID.
func (uc *RestoreTarget) Execute(ctx context.Context, adminID, takedownID string) (domain.Takedown, error) {
	td, err := uc.takedowns.Get(ctx, takedownID)
	if err != nil {
		return domain.Takedown{}, err
	}
	if err := td.CheckRestorable(); err != nil {
		return domain.Takedown{}, err
	}
	// The owner may delete hidden content (D23). Then there is nothing to put
	// back, so the takedown and the target's open reports are only closed.
	exists, err := uc.targetExists(ctx, td.Target)
	if err != nil {
		return domain.Takedown{}, err
	}

	var restored domain.Takedown
	err = uc.txm.WithinTx(ctx, func(ctx context.Context) error {
		if exists {
			if err := uc.targets.restore(ctx, td.Target, uc.wordCap); err != nil {
				return err
			}
		}
		if restored, err = uc.takedowns.MarkRestored(ctx, td.ID, adminID); err != nil {
			return err
		}
		_, err = uc.reports.Resolve(ctx, td.Target, domain.StatusDismissed, adminID)
		return err
	})
	return restored, err
}

// targetExists reports whether the takedown's target is still stored.
func (uc *RestoreTarget) targetExists(ctx context.Context, ref domain.TargetRef) (bool, error) {
	_, err := uc.targets.load(ctx, ref)
	switch {
	case errors.Is(err, kernel.ErrNotFound):
		return false, nil
	case err != nil:
		return false, err
	default:
		return true, nil
	}
}
