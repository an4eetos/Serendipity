// Package postgres implements the moderation context's repositories on
// Postgres.
package postgres

import (
	"context"

	kernelpg "github.com/an4eetos/serendipity/backend/internal/kernel/adapters/postgres"
	"github.com/an4eetos/serendipity/backend/internal/moderation/domain"
)

const reportColumns = `r.id, r.reporter_id, r.target_kind, r.target_id, r.target_owner_id, p.username,
	r.reason, r.note, r.status, r.created_at`

func scanReport(row interface{ Scan(...any) error }) (domain.Report, error) {
	var r domain.Report
	err := row.Scan(&r.ID, &r.ReporterID, &r.Target.Kind, &r.Target.ID, &r.OwnerID, &r.OwnerUsername,
		&r.Reason, &r.Note, &r.Status, &r.CreatedAt)
	return r, err
}

// ReportRepository implements ports.ReportRepository.
type ReportRepository struct {
	db *kernelpg.DB
}

// NewReportRepository returns a ReportRepository on db.
func NewReportRepository(db *kernelpg.DB) *ReportRepository {
	return &ReportRepository{db: db}
}

// Create implements ports.ReportRepository.
func (r *ReportRepository) Create(ctx context.Context, rep domain.Report) (domain.Report, error) {
	err := r.db.QueryRow(ctx, `
		insert into reports (reporter_id, target_kind, target_id, target_owner_id, reason, note)
		values ($1, $2, $3, $4, $5::report_reason, $6)
		returning id, status, created_at`,
		rep.ReporterID, rep.Target.Kind, rep.Target.ID, rep.OwnerID, rep.Reason, rep.Note,
	).Scan(&rep.ID, &rep.Status, &rep.CreatedAt)
	switch {
	case kernelpg.IsUniqueViolation(err):
		return domain.Report{}, domain.ErrAlreadyReported
	case kernelpg.IsCheckViolation(err):
		return domain.Report{}, domain.ErrOwnContent
	case err != nil:
		return domain.Report{}, err
	default:
		return rep, nil
	}
}

// ListOpen implements ports.ReportRepository.
func (r *ReportRepository) ListOpen(ctx context.Context, limit int) ([]domain.Report, error) {
	rows, err := r.db.Query(ctx, `
		select `+reportColumns+`
		from reports r join profiles p on p.id = r.target_owner_id
		where r.status = 'open'
		order by r.created_at
		limit $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []domain.Report{}
	for rows.Next() {
		rep, err := scanReport(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, rep)
	}
	return out, rows.Err()
}

// Resolve implements ports.ReportRepository.
func (r *ReportRepository) Resolve(
	ctx context.Context, target domain.TargetRef, status domain.ReportStatus, adminID string,
) (int, error) {
	tag, err := r.db.Exec(ctx, `
		update reports set
			status      = $3::report_status,
			resolved_by = $4,
			resolved_at = now(),
			updated_at  = now()
		where target_kind = $1 and target_id = $2 and status = 'open'`,
		target.Kind, target.ID, status, adminID)
	if err != nil {
		return 0, err
	}
	return int(tag.RowsAffected()), nil
}
