package postgres

import (
	"context"
	"time"

	"github.com/an4eetos/serendipity/backend/internal/discussion/ports"
	kernelpg "github.com/an4eetos/serendipity/backend/internal/kernel/adapters/postgres"
)

// ReplyLog implements ports.ReplyLog on the character_replies table.
type ReplyLog struct {
	db *kernelpg.DB
}

var _ ports.ReplyLog = (*ReplyLog)(nil)

// NewReplyLog returns the ReplyLog.
func NewReplyLog(db *kernelpg.DB) *ReplyLog {
	return &ReplyLog{db: db}
}

// Lock implements ports.ReplyLog with a transaction-scoped advisory lock,
// which works behind the transaction pooler.
func (l *ReplyLog) Lock(ctx context.Context, userID string) error {
	_, err := l.db.Exec(ctx, `select pg_advisory_xact_lock(hashtextextended('character_replies:' || $1, 0))`, userID)
	return err
}

// CountSince implements ports.ReplyLog.
func (l *ReplyLog) CountSince(ctx context.Context, userID string, window time.Duration) (int, error) {
	var n int
	err := l.db.QueryRow(ctx, `select count(*) from character_replies
		where user_id = $1 and created_at > now() - make_interval(secs => $2)`,
		userID, window.Seconds()).Scan(&n)
	return n, err
}

// Add implements ports.ReplyLog.
func (l *ReplyLog) Add(ctx context.Context, userID string, keep time.Duration) error {
	_, err := l.db.Exec(ctx, `
		with pruned as (
			delete from character_replies
			where user_id = $1 and created_at <= now() - make_interval(secs => $2)
		)
		insert into character_replies (user_id) values ($1)`,
		userID, keep.Seconds())
	return err
}
