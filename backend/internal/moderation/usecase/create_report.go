package usecase

import (
	"context"

	discussionports "github.com/an4eetos/serendipity/backend/internal/discussion/ports"
	kernelports "github.com/an4eetos/serendipity/backend/internal/kernel/ports"
	"github.com/an4eetos/serendipity/backend/internal/moderation/domain"
	"github.com/an4eetos/serendipity/backend/internal/moderation/ports"
	quoteports "github.com/an4eetos/serendipity/backend/internal/quote/ports"
)

// CreateReport lets a signed-in reader ask admins to look at someone else's
// public quote or discussion.
type CreateReport struct {
	txm      kernelports.TxManager
	reports  ports.ReportRepository
	profiles ports.ProfileVisibility
	targets  targets
}

// NewCreateReport returns the CreateReport usecase.
func NewCreateReport(
	txm kernelports.TxManager,
	reports ports.ReportRepository,
	profiles ports.ProfileVisibility,
	quotes quoteports.QuoteModeration,
	conversations discussionports.ConversationModeration,
) *CreateReport {
	return &CreateReport{
		txm: txm, reports: reports, profiles: profiles,
		targets: targets{quotes: quotes, conversations: conversations},
	}
}

// Execute stores reporterID's open report on ref.
func (uc *CreateReport) Execute(
	ctx context.Context, reporterID string, ref domain.TargetRef, reason domain.Reason, note string,
) (domain.Report, error) {
	r, err := domain.NewReport(reporterID, ref, reason, note)
	if err != nil {
		return domain.Report{}, err
	}
	t, err := uc.targets.load(ctx, ref)
	if err != nil {
		return domain.Report{}, err
	}
	if t.Public {
		// Readers see content only on public profiles.
		profilePublic, err := uc.profiles.IsPublic(ctx, t.OwnerID)
		if err != nil {
			return domain.Report{}, err
		}
		t = t.OnProfile(profilePublic)
	}
	if err := t.CheckReportable(reporterID); err != nil {
		return domain.Report{}, err
	}
	// The stored ref is the target's own, so that every report on it
	// groups and resolves together however the reader spelled the id.
	r.Target, r.OwnerID = t.Ref, t.OwnerID

	var created domain.Report
	err = uc.txm.WithinTx(ctx, func(ctx context.Context) error {
		created, err = uc.reports.Create(ctx, r)
		return err
	})
	return created, err
}
