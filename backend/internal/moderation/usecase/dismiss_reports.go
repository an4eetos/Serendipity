package usecase

import (
	"context"

	kernelports "github.com/an4eetos/serendipity/backend/internal/kernel/ports"
	"github.com/an4eetos/serendipity/backend/internal/moderation/domain"
	"github.com/an4eetos/serendipity/backend/internal/moderation/ports"
)

// DismissReports closes a target's open reports and leaves it up.
type DismissReports struct {
	txm     kernelports.TxManager
	reports ports.ReportRepository
}

// NewDismissReports returns the DismissReports usecase.
func NewDismissReports(txm kernelports.TxManager, reports ports.ReportRepository) *DismissReports {
	return &DismissReports{txm: txm, reports: reports}
}

// Execute returns how many reports it closed, or domain.ErrNoOpenReports
// when there were none.
func (uc *DismissReports) Execute(ctx context.Context, adminID string, ref domain.TargetRef) (int, error) {
	if err := ref.Validate(); err != nil {
		return 0, err
	}
	var n int
	err := uc.txm.WithinTx(ctx, func(ctx context.Context) error {
		var err error
		if n, err = uc.reports.Resolve(ctx, ref, domain.StatusDismissed, adminID); err != nil {
			return err
		}
		if n == 0 {
			return domain.ErrNoOpenReports
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	return n, nil
}
