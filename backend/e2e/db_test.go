//go:build e2e

package e2e

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// countedTables are the tables whose row counts must be back to the
// baseline after cleanup, in the column order of countRowsSQL.
var countedTables = []string{
	"auth.users", "profiles", "works", "library_items", "quotes", "conversations",
	"messages", "reports", "takedowns", "admins", "storage.objects", "personas",
	"character_replies",
}

const countRowsSQL = `
	select (select count(*) from auth.users),
	       (select count(*) from public.profiles),
	       (select count(*) from public.works),
	       (select count(*) from public.library_items),
	       (select count(*) from public.quotes),
	       (select count(*) from public.conversations),
	       (select count(*) from public.messages),
	       (select count(*) from public.reports),
	       (select count(*) from public.takedowns),
	       (select count(*) from public.admins),
	       (select count(*) from storage.objects),
	       (select count(*) from public.personas),
	       (select count(*) from public.character_replies)`

// rowCounts maps a table name to its row count.
type rowCounts map[string]int64

// countRows counts the rows of every counted table.
func countRows(ctx context.Context, db *pgxpool.Pool) (rowCounts, error) {
	vals := make([]int64, len(countedTables))
	dest := make([]any, len(vals))
	for i := range vals {
		dest[i] = &vals[i]
	}
	if err := db.QueryRow(ctx, countRowsSQL).Scan(dest...); err != nil {
		return nil, err
	}
	counts := rowCounts{}
	for i, name := range countedTables {
		counts[name] = vals[i]
	}
	return counts, nil
}

// diff lists the tables whose count differs from want.
func (c rowCounts) diff(want rowCounts) []string {
	var out []string
	for _, name := range countedTables {
		if c[name] != want[name] {
			out = append(out, fmt.Sprintf("%s: %d, baseline %d", name, c[name], want[name]))
		}
	}
	return out
}

// makeAdmin adds userID to the admins table, which no API route can do.
func makeAdmin(ctx context.Context, db *pgxpool.Pool, userID string) error {
	_, err := db.Exec(ctx, `insert into public.admins (user_id) values ($1)`, userID)
	return err
}

// insertAnsweredConversation stores a discussion of quoteID with the
// Socrates preset, with a reader message and a character reply, and returns
// its id. The API can only create one by calling Anthropic.
func insertAnsweredConversation(ctx context.Context, db *pgxpool.Pool, userID, quoteID string) (string, error) {
	var socrates string
	err := db.QueryRow(ctx, `select id from public.personas where slug = 'socrates' and is_preset`).Scan(&socrates)
	if err != nil {
		return "", fmt.Errorf("find the Socrates preset: %w", err)
	}
	return insertAnsweredConversationWith(ctx, db, userID, quoteID, socrates)
}

// insertAnsweredConversationWith is insertAnsweredConversation with
// personaID, a preset or the reader's own character. The insert triggers
// still check that the character is usable.
func insertAnsweredConversationWith(ctx context.Context, db *pgxpool.Pool, userID, quoteID, personaID string) (string, error) {
	var id string
	err := pgx.BeginFunc(ctx, db, func(tx pgx.Tx) error {
		err := tx.QueryRow(ctx, `
			insert into public.conversations (user_id, quote_id, persona_id)
			values ($1, $2, $3) returning id`, userID, quoteID, personaID).Scan(&id)
		if err != nil {
			return fmt.Errorf("insert conversation: %w", err)
		}
		_, err = tx.Exec(ctx, `
			insert into public.messages (conversation_id, role, content)
			values ($1, 'user', 'e2e: what do you make of this?'),
			       ($1, 'assistant', 'e2e: what do you think it means?')`, id)
		return err
	})
	return id, err
}

// messageCount counts a discussion's stored messages.
func messageCount(ctx context.Context, db *pgxpool.Pool, conversationID string) (int, error) {
	var n int
	err := db.QueryRow(ctx, `select count(*) from public.messages where conversation_id = $1`, conversationID).Scan(&n)
	return n, err
}

// deleteReport removes one report row. It undoes a report the API should
// have refused, so that the rest of the scenario still runs.
func deleteReport(ctx context.Context, db *pgxpool.Pool, id string) error {
	_, err := db.Exec(ctx, `delete from public.reports where id = $1`, id)
	return err
}

// taggedUserIDs returns the auth users whose email carries the run's tag.
func taggedUserIDs(ctx context.Context, db *pgxpool.Pool, tag string) ([]string, error) {
	return queryStrings(ctx, db, `select id::text from auth.users where email like $1`, tag+"-%")
}

// taggedWorkIDs returns the works whose title carries the run's tag.
func taggedWorkIDs(ctx context.Context, db *pgxpool.Pool, tag string) ([]string, error) {
	return queryStrings(ctx, db, `select id::text from public.works where title like $1`, tag+" %")
}

// objectNames returns the books-bucket objects in the folders of userIDs.
func objectNames(ctx context.Context, db *pgxpool.Pool, userIDs []string) ([]string, error) {
	return queryStrings(ctx, db, `
		select name from storage.objects
		where bucket_id = 'books' and split_part(name, '/', 1) = any($1::text[])`, userIDs)
}

// deleteWorks deletes works by id and returns how many were deleted.
func deleteWorks(ctx context.Context, db *pgxpool.Pool, ids []string) (int64, error) {
	tag, err := db.Exec(ctx, `delete from public.works where id = any($1::uuid[])`, ids)
	return tag.RowsAffected(), err
}

func queryStrings(ctx context.Context, db *pgxpool.Pool, sql string, arg any) ([]string, error) {
	rows, err := db.Query(ctx, sql, arg)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowTo[string])
}
