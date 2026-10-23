package postgres

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	catalogpg "github.com/an4eetos/serendipity/backend/internal/catalog/adapters/postgres"
	catalog "github.com/an4eetos/serendipity/backend/internal/catalog/domain"
	kernelpg "github.com/an4eetos/serendipity/backend/internal/kernel/adapters/postgres"
	"github.com/an4eetos/serendipity/backend/internal/kernel/adapters/postgres/pgtest"
	"github.com/an4eetos/serendipity/backend/internal/quote/domain"
	"github.com/an4eetos/serendipity/backend/internal/quote/usecase"
)

// The concurrency tests need transactions that really run side by side, so
// unlike the other adapter tests they commit. Their rows carry a unique tag
// and are deleted, by id, when the test ends.

// raceCap is the cap of the race: each passage fits under it alone, both
// together don't.
const raceCap = 10

// race is two readers' private quotes of one work, committed.
type race struct {
	pool   *pgxpool.Pool
	db     *kernelpg.DB
	users  []string
	workID string
	quotes []domain.Quote
}

func newRace(t *testing.T) *race {
	t.Helper()
	pool, err := pgxpool.New(context.Background(), pgtest.URL(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	r := &race{pool: pool, db: kernelpg.NewDB(pool)}
	// Registered before anything is created, so it runs however the test
	// ends, and after the test's context is canceled.
	t.Cleanup(func() { r.cleanUp(t) })

	ctx := context.Background()
	var tag string
	if err := r.db.QueryRow(ctx, `select 'pgtest-race-' || substr(md5(gen_random_uuid()::text), 1, 12)`).Scan(&tag); err != nil {
		t.Fatal(err)
	}
	for _, reader := range []string{"a", "b"} {
		var id string
		err := r.db.QueryRow(ctx, `insert into auth.users (id, email) values (gen_random_uuid(), $1) returning id`,
			tag+"-"+reader+"@example.com").Scan(&id)
		if err != nil {
			t.Fatal(err)
		}
		r.users = append(r.users, id)
	}
	work, err := catalogpg.NewWorkRepository(r.db).Ensure(ctx, catalog.Work{Title: tag}, r.users[0])
	if err != nil {
		t.Fatal(err)
	}
	r.workID = work.ID
	quotes := NewQuoteRepository(r.db)
	for i, text := range []string{"One two three four five six", "Seven eight nine ten eleven twelve"} {
		q, err := quotes.Create(ctx, r.users[i], domain.Draft{
			WorkID: r.workID, Text: text, Location: "p. 1", Reaction: "Mine.", Source: domain.SourceUpload,
		})
		if err != nil {
			t.Fatal(err)
		}
		r.quotes = append(r.quotes, q)
	}
	return r
}

// cleanUp deletes exactly what newRace created. Deleting the users cascades
// to their profiles and quotes; the work goes last, once nothing cites it.
func (r *race) cleanUp(t *testing.T) {
	ctx := context.Background()
	if len(r.users) > 0 {
		if _, err := r.db.Exec(ctx, `delete from auth.users where id = any($1::uuid[])`, r.users); err != nil {
			t.Errorf("cleanup: deleting the test users: %v", err)
		}
	}
	if r.workID != "" {
		if _, err := r.db.Exec(ctx, `delete from works where id = $1`, r.workID); err != nil {
			t.Errorf("cleanup: deleting the test work: %v", err)
		}
	}
	var left int
	err := r.db.QueryRow(ctx, `
		select (select count(*) from auth.users where id = any($1::uuid[]))
			+ (select count(*) from profiles where id = any($1::uuid[]))
			+ (select count(*) from quotes where work_id = $2::uuid)
			+ (select count(*) from works where id = $2::uuid)`, r.users, nullIfEmpty(r.workID)).Scan(&left)
	if err != nil || left != 0 {
		t.Errorf("cleanup left %d rows behind (%v)", left, err)
	}
}

func nullIfEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// reset makes both quotes private and shown again, committed.
func (r *race) reset(t *testing.T) {
	t.Helper()
	_, err := r.db.Exec(context.Background(), `
		update quotes set visibility = 'private', passage_hidden = false where id = any($1::uuid[])`,
		[]string{r.quotes[0].ID, r.quotes[1].ID})
	if err != nil {
		t.Fatal(err)
	}
}

// outcome reads the committed coverage of the work and how many of the two
// quotes are public with their passage collapsed.
func (r *race) outcome(t *testing.T) (covered, public, collapsed int) {
	t.Helper()
	err := r.db.QueryRow(context.Background(), `
		select
			coalesce((select sum(d.words) from (
				select max(q.passage_words) as words from quotes q
				where q.work_id = $1 and q.visibility = 'public' and q.removed_at is null and not q.passage_hidden
				group by q.passage_hash) d), 0),
			(select count(*) from quotes where work_id = $1 and visibility = 'public'),
			(select count(*) from quotes where work_id = $1 and visibility = 'public' and passage_hidden)`,
		r.workID).Scan(&covered, &public, &collapsed)
	if err != nil {
		t.Fatal(err)
	}
	return covered, public, collapsed
}

func (r *race) check(t *testing.T, round int) {
	t.Helper()
	covered, public, collapsed := r.outcome(t)
	if covered > raceCap || public != 2 || collapsed != 1 {
		t.Errorf("round %d: coverage %d (cap %d), %d public, %d collapsed; want both public, exactly one collapsed",
			round, covered, raceCap, public, collapsed)
	}
}

// together runs fns at the same moment and returns their errors.
func together(fns ...func() error) []error {
	errs := make([]error, len(fns))
	start := make(chan struct{})
	var wg sync.WaitGroup
	for i, fn := range fns {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			errs[i] = fn()
		}()
	}
	close(start)
	wg.Wait()
	return errs
}

const raceRounds = 25

func TestConcurrentPublishesThroughUpdateQuote(t *testing.T) {
	r := newRace(t)
	uc := usecase.NewUpdateQuote(kernelpg.NewTxManager(r.db), NewQuoteRepository(r.db), NewCoverageRepository(r.db), raceCap)
	publish := func(i int) func() error {
		return func() error {
			_, err := uc.Execute(t.Context(), r.users[i], r.quotes[i].ID, domain.Patch{Visibility: ptr(domain.VisibilityPublic)})
			return err
		}
	}
	for round := range raceRounds {
		r.reset(t)
		for _, err := range together(publish(0), publish(1)) {
			if err != nil {
				t.Fatalf("round %d: %v", round, err)
			}
		}
		r.check(t, round)
	}
}

// directPublish makes the quote public the way a write that bypasses the API
// would, with the cap the API would set: only the trigger decides.
func (r *race) directPublish(ctx context.Context, id string) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(context.WithoutCancel(ctx)) }()
	if _, err := tx.Exec(ctx, `select set_config('serendipity.public_word_cap', '10', true)`); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `update quotes set visibility = 'public' where id = $1`, id); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func TestConcurrentPublishesThroughTheTrigger(t *testing.T) {
	r := newRace(t)
	for round := range raceRounds {
		r.reset(t)
		errs := together(
			func() error { return r.directPublish(t.Context(), r.quotes[0].ID) },
			func() error { return r.directPublish(t.Context(), r.quotes[1].ID) },
		)
		for _, err := range errs {
			if err != nil {
				t.Fatalf("round %d: %v", round, err)
			}
		}
		r.check(t, round)
	}
}

// TestSecondPublishWaitsForTheFirst forces the interleaving the races above
// only make likely: the second publish starts while the first holds the
// work's lock, waits, and then sees the first passage.
func TestSecondPublishWaitsForTheFirst(t *testing.T) {
	r := newRace(t)
	r.reset(t)
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()

	first, err := r.pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = first.Rollback(context.WithoutCancel(ctx)) }()
	if _, err := first.Exec(ctx, `select set_config('serendipity.public_word_cap', '10', true)`); err != nil {
		t.Fatal(err)
	}
	if _, err := first.Exec(ctx, `update quotes set visibility = 'public' where id = $1`, r.quotes[0].ID); err != nil {
		t.Fatal(err)
	}

	second, err := r.pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = second.Rollback(context.WithoutCancel(ctx)) }()
	var pid int
	if err := second.QueryRow(ctx, `select pg_backend_pid()`).Scan(&pid); err != nil {
		t.Fatal(err)
	}
	if _, err := second.Exec(ctx, `select set_config('serendipity.public_word_cap', '10', true)`); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() {
		_, err := second.Exec(ctx, `update quotes set visibility = 'public' where id = $1`, r.quotes[1].ID)
		done <- err
	}()

	// The second update must be waiting on a lock, not done.
	waiting := false
	for deadline := time.Now().Add(10 * time.Second); time.Now().Before(deadline) && !waiting; {
		select {
		case err := <-done:
			t.Fatalf("the second publish finished while the first held the work's lock (err %v)", err)
		case <-time.After(20 * time.Millisecond):
		}
		var event *string
		if err := r.db.QueryRow(ctx, `select wait_event_type from pg_stat_activity where pid = $1`, pid).Scan(&event); err != nil {
			t.Fatal(err)
		}
		waiting = event != nil && *event == "Lock"
	}
	if !waiting {
		t.Fatal("the second publish never waited on a lock")
	}

	if err := first.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if err := second.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	covered, public, collapsed := r.outcome(t)
	var secondHidden bool
	if err := r.db.QueryRow(ctx, `select passage_hidden from quotes where id = $1`, r.quotes[1].ID).Scan(&secondHidden); err != nil {
		t.Fatal(err)
	}
	if covered != 6 || public != 2 || collapsed != 1 || !secondHidden {
		t.Errorf("coverage %d, %d public, %d collapsed, second collapsed %v; want 6, 2, 1, true",
			covered, public, collapsed, secondHidden)
	}
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		t.Error("timed out")
	}
}
