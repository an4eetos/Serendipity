package usecase

import (
	"context"

	kernelports "github.com/an4eetos/serendipity/backend/internal/kernel/ports"
	"github.com/an4eetos/serendipity/backend/internal/quote/domain"
	"github.com/an4eetos/serendipity/backend/internal/quote/ports"
)

// UpdateQuote edits the reader's reaction or publishes the quote. Publishing
// collapses the passage when the work's public coverage cap is reached.
type UpdateQuote struct {
	txm      kernelports.TxManager
	quotes   ports.QuoteRepository
	coverage ports.CoverageRepository
	wordCap  domain.CoverageCap
}

func NewUpdateQuote(txm kernelports.TxManager, quotes ports.QuoteRepository,
	coverage ports.CoverageRepository, wordCap domain.CoverageCap) *UpdateQuote {
	return &UpdateQuote{txm: txm, quotes: quotes, coverage: coverage, wordCap: wordCap}
}

// Execute returns the stored quote, so when the database's coverage check
// disagrees with this one, its decision is what comes back.
func (uc *UpdateQuote) Execute(ctx context.Context, userID, id string, p domain.Patch) (domain.Quote, error) {
	// Look the reader's id up before the transaction: a malformed one would
	// abort it.
	if _, err := uc.quotes.Get(ctx, userID, id); err != nil {
		return domain.Quote{}, err
	}
	// Only the publishing rules below decide whether the passage is hidden.
	p.PassageHidden = nil
	var q domain.Quote
	err := uc.txm.WithinTx(ctx, func(ctx context.Context) error {
		current, err := uc.quotes.Get(ctx, userID, id)
		if err != nil {
			return err
		}
		if err := current.CheckPatch(p); err != nil {
			return err
		}
		if current.Publishes(p) {
			hidden, err := uc.hidesPassage(ctx, current)
			if err != nil {
				return err
			}
			p.PassageHidden = &hidden
		}
		q, err = uc.quotes.Update(ctx, userID, id, p)
		return err
	})
	return q, err
}

// hidesPassage decides, under the work's lock, whether publishing q would take
// the work past its cap. It also gives the database's coverage check the same
// default cap for the rest of the transaction.
func (uc *UpdateQuote) hidesPassage(ctx context.Context, q domain.Quote) (bool, error) {
	if err := uc.coverage.SetCap(ctx, uc.wordCap); err != nil {
		return false, err
	}
	if err := uc.coverage.LockWork(ctx, q.WorkID); err != nil {
		return false, err
	}
	cov, err := uc.coverage.Coverage(ctx, q.WorkID, q.ID)
	if err != nil {
		return false, err
	}
	return !cov.ShowsPassage(q, uc.wordCap), nil
}
