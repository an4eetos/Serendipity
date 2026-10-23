package postgres

import (
	"context"
	"strconv"

	kernelpg "github.com/an4eetos/serendipity/backend/internal/kernel/adapters/postgres"
	"github.com/an4eetos/serendipity/backend/internal/quote/domain"
	"github.com/an4eetos/serendipity/backend/internal/quote/ports"
)

// CoverageRepository implements ports.CoverageRepository.
type CoverageRepository struct {
	db *kernelpg.DB
}

var _ ports.CoverageRepository = (*CoverageRepository)(nil)

func NewCoverageRepository(db *kernelpg.DB) *CoverageRepository {
	return &CoverageRepository{db: db}
}

// setCoverageCap makes wordCap the default cap of the coverage trigger
// (quotes_enforce_public_coverage) for the rest of the caller's transaction.
// The setting is transaction-local, so it holds behind any connection pooler
// and never leaks to the connection's next transaction.
func setCoverageCap(ctx context.Context, db *kernelpg.DB, wordCap domain.CoverageCap) error {
	var setting string
	return db.QueryRow(ctx, `select set_config('serendipity.public_word_cap', $1, true)`,
		strconv.Itoa(int(wordCap))).Scan(&setting)
}

func (r *CoverageRepository) SetCap(ctx context.Context, wordCap domain.CoverageCap) error {
	return setCoverageCap(ctx, r.db, wordCap)
}

// LockWork takes the same row lock as the coverage trigger. NO KEY UPDATE
// doesn't block inserting quotes of the work.
func (r *CoverageRepository) LockWork(ctx context.Context, workID string) error {
	var locked int
	return r.db.QueryRow(ctx, `select 1 from works where id = $1 for no key update`, workID).Scan(&locked)
}

func (r *CoverageRepository) Coverage(ctx context.Context, workID, quoteID string) (domain.Coverage, error) {
	var c domain.Coverage
	err := r.db.QueryRow(ctx, `
		select
			coalesce((select sum(d.words) from (
				select max(q.passage_words) as words from quotes q
				where q.work_id = $1 and q.id <> $2
					and q.visibility = 'public' and q.removed_at is null and not q.passage_hidden
				group by q.passage_hash) d), 0),
			exists (select 1 from quotes q
				where q.work_id = $1 and q.id <> $2
					and q.passage_hash = (select m.passage_hash from quotes m where m.id = $2)
					and q.visibility = 'public' and q.removed_at is null and not q.passage_hidden),
			(select w.public_word_cap from works w where w.id = $1)`,
		workID, quoteID).Scan(&c.Words, &c.IdenticalShown, &c.CapOverride)
	return c, err
}
