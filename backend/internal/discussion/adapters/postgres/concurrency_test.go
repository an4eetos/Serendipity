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
	"github.com/an4eetos/serendipity/backend/internal/discussion/domain"
	"github.com/an4eetos/serendipity/backend/internal/discussion/usecase"
	kernelpg "github.com/an4eetos/serendipity/backend/internal/kernel/adapters/postgres"
	"github.com/an4eetos/serendipity/backend/internal/kernel/adapters/postgres/pgtest"
	quotepg "github.com/an4eetos/serendipity/backend/internal/quote/adapters/postgres"
	quote "github.com/an4eetos/serendipity/backend/internal/quote/domain"
)

// The concurrency tests need transactions that really run side by side, so
// unlike the other adapter tests they commit, as the quote context's do.
// Their rows belong to a tagged user and work, deleted when the test ends.

// committed is a reader with a quote, committed.
type committed struct {
	t      *testing.T
	pool   *pgxpool.Pool
	db     *kernelpg.DB
	txm    *kernelpg.TxManager
	repo   *PersonaRepository
	convs  *ConversationRepository
	user   string
	workID string
	quote  quote.Quote
}

func newCommitted(t *testing.T) *committed {
	t.Helper()
	pool, err := pgxpool.New(context.Background(), pgtest.URL(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	c := &committed{t: t, pool: pool, db: kernelpg.NewDB(pool)}
	c.txm, c.repo, c.convs = kernelpg.NewTxManager(c.db), NewPersonaRepository(c.db), NewConversationRepository(c.db)
	// Registered before anything is created, so it runs however the test
	// ends.
	t.Cleanup(c.cleanUp)

	ctx := context.Background()
	var tag string
	if err := c.db.QueryRow(ctx, `select 'pgtest-chars-' || substr(md5(gen_random_uuid()::text), 1, 12)`).Scan(&tag); err != nil {
		t.Fatal(err)
	}
	if err := c.db.QueryRow(ctx, `insert into auth.users (id, email) values (gen_random_uuid(), $1) returning id`,
		tag+"@example.com").Scan(&c.user); err != nil {
		t.Fatal(err)
	}
	work, err := catalogpg.NewWorkRepository(c.db).Ensure(ctx, catalog.Work{Title: tag}, c.user)
	if err != nil {
		t.Fatal(err)
	}
	c.workID = work.ID
	if c.quote, err = quotepg.NewQuoteRepository(c.db).Create(ctx, c.user, quote.Draft{
		WorkID: c.workID, Text: "One two three", Location: "p. 1", Reaction: "Mine.", Source: quote.SourceUpload,
	}); err != nil {
		t.Fatal(err)
	}
	return c
}

// cleanUp deletes exactly what newCommitted and the test created. Deleting
// the user cascades to the profile, characters, quotes and discussions; the
// work goes last, once nothing cites it.
func (c *committed) cleanUp() {
	ctx := context.Background()
	if c.user != "" {
		if _, err := c.db.Exec(ctx, `delete from auth.users where id = $1`, c.user); err != nil {
			c.t.Errorf("cleanup: deleting the test user: %v", err)
		}
	}
	if c.workID != "" {
		if _, err := c.db.Exec(ctx, `delete from works where id = $1`, c.workID); err != nil {
			c.t.Errorf("cleanup: deleting the test work: %v", err)
		}
	}
	var left int
	err := c.db.QueryRow(ctx, `
		select (select count(*) from auth.users where id = $1::uuid)
			+ (select count(*) from profiles where id = $1::uuid)
			+ (select count(*) from personas where owner_id = $1::uuid)
			+ (select count(*) from conversations where user_id = $1::uuid)
			+ (select count(*) from quotes where work_id = $2::uuid)
			+ (select count(*) from works where id = $2::uuid)`, nullIfEmpty(c.user), nullIfEmpty(c.workID)).Scan(&left)
	if err != nil || left != 0 {
		c.t.Errorf("cleanup left %d rows behind (%v)", left, err)
	}
}

func nullIfEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func (c *committed) character(name string) domain.Persona {
	c.t.Helper()
	p, err := c.repo.Create(context.Background(), c.user,
		domain.CharacterDraft{Name: name, Prompt: "You are " + name + "."}, domain.SlugBase(name))
	if err != nil {
		c.t.Fatal(err)
	}
	return p
}

// waitingReply is a discussion with p whose reader message is stored and
// whose reply is being generated.
func (c *committed) waitingReply(p domain.Persona) string {
	c.t.Helper()
	ctx := context.Background()
	id, err := c.convs.Create(ctx, c.user, c.quote.ID, p.ID)
	if err != nil {
		c.t.Fatal(err)
	}
	if _, err := c.convs.AppendMessage(ctx, id, domain.RoleUser, "What do you make of it?"); err != nil {
		c.t.Fatal(err)
	}
	return id
}

// hold begins a transaction that runs sql and stays open, holding its row
// locks, until the returned commit is called. It returns the transaction's
// backend pid.
func (c *committed) hold(ctx context.Context, sql string, args ...any) (pid int, commit func() error) {
	c.t.Helper()
	tx, err := c.pool.Begin(ctx)
	if err != nil {
		c.t.Fatal(err)
	}
	c.t.Cleanup(func() { _ = tx.Rollback(context.Background()) })
	if err := tx.QueryRow(ctx, `select pg_backend_pid()`).Scan(&pid); err != nil {
		c.t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, sql, args...); err != nil {
		c.t.Fatal(err)
	}
	return pid, func() error { return tx.Commit(ctx) }
}

// whileBlocked runs fn in the background and returns once it is waiting on
// a lock that holder holds. The returned channel gets fn's error.
func (c *committed) whileBlocked(ctx context.Context, holder int, fn func() error) <-chan error {
	c.t.Helper()
	done := make(chan error, 1)
	go func() { done <- fn() }()
	for deadline := time.Now().Add(10 * time.Second); time.Now().Before(deadline); {
		select {
		case err := <-done:
			c.t.Fatalf("finished while the other transaction held its lock (err %v)", err)
		case <-time.After(20 * time.Millisecond):
		}
		var blocked bool
		if err := c.db.QueryRow(ctx, `select exists (select 1 from pg_stat_activity where $1 = any(pg_blocking_pids(pid)))`,
			holder).Scan(&blocked); err != nil {
			c.t.Fatal(err)
		}
		if blocked {
			return done
		}
	}
	c.t.Fatal("never waited on the other transaction's lock")
	return nil
}

// appendReply stores the character's reply the way replier.answer does.
func (c *committed) appendReply(ctx context.Context, convID string) func() error {
	return func() error {
		return c.txm.WithinTx(ctx, func(ctx context.Context) error {
			_, err := c.convs.AppendMessage(ctx, convID, domain.RoleAssistant, "A late reply.")
			return err
		})
	}
}

func (c *committed) replies(convID string) int {
	c.t.Helper()
	var n int
	if err := c.db.QueryRow(context.Background(), `select count(*) from messages where conversation_id = $1 and role = 'assistant'`,
		convID).Scan(&n); err != nil {
		c.t.Fatal(err)
	}
	return n
}

func testContext(t *testing.T) context.Context {
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	t.Cleanup(cancel)
	return ctx
}

// R-RACE: a state change committed while the reply's insert waits on it
// refuses the reply with the matching error, and nothing is stored.
func TestStateChangeCommittedDuringAReply(t *testing.T) {
	tests := []struct {
		name string
		sql  string
		on   func(p domain.Persona, conv string) string
		want error
	}{
		{"archive", `update personas set archived_at = now(), updated_at = now() where id = $1`,
			func(p domain.Persona, _ string) string { return p.ID }, domain.ErrCharacterArchived},
		{"character removal", `update personas set removed_at = now(), removed_reason = 'spam' where id = $1`,
			func(p domain.Persona, _ string) string { return p.ID }, domain.ErrCharacterRemoved},
		{"(D13) discussion removal", `update conversations set removed_at = now(), removed_reason = 'spam' where id = $1`,
			func(_ domain.Persona, conv string) string { return conv }, domain.ErrConversationRemoved},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := newCommitted(t)
			ctx := testContext(t)
			p := c.character("Ada")
			conv := c.waitingReply(p)

			holder, commit := c.hold(ctx, tt.sql, tt.on(p, conv))
			done := c.whileBlocked(ctx, holder, c.appendReply(ctx, conv))
			if err := commit(); err != nil {
				t.Fatal(err)
			}
			if err := <-done; !errors.Is(err, tt.want) {
				t.Errorf("reply insert = %v, want %v", err, tt.want)
			}
			if n := c.replies(conv); n != 0 {
				t.Errorf("%d replies stored, want none", n)
			}
		})
	}
}

// R-RACE, the other order: a reply being inserted holds the character, so
// archiving it waits, and both end consistently: the reply is kept and the
// character is archived after it.
func TestArchiveWaitsForAReplyBeingStored(t *testing.T) {
	c := newCommitted(t)
	ctx := testContext(t)
	p := c.character("Ada")
	conv := c.waitingReply(p)

	holder, commit := c.hold(ctx, `insert into messages (conversation_id, role, content) values ($1, 'assistant', 'In time.')`, conv)
	archive := usecase.NewArchiveCharacter(c.txm, c.repo)
	var archived domain.Persona
	done := c.whileBlocked(ctx, holder, func() error {
		var err error
		archived, err = archive.Execute(ctx, c.user, p.ID)
		return err
	})
	if err := commit(); err != nil {
		t.Fatal(err)
	}
	if err := <-done; err != nil || archived.Status() != domain.PersonaArchived {
		t.Fatalf("archive = %#v, %v", archived, err)
	}
	if n := c.replies(conv); n != 1 {
		t.Errorf("%d replies stored, want the one inserted first", n)
	}
	if err := c.appendReply(ctx, conv)(); !errors.Is(err, domain.ErrCharacterArchived) {
		t.Errorf("a reply after the archive = %v, want ErrCharacterArchived", err)
	}
}

// R-LOCK: a rename never lands after the first discussion exists. When the
// discussion is being created, the rename waits and then sees it.
func TestRenameWaitsForTheFirstDiscussion(t *testing.T) {
	t.Run("through UpdateCharacter", func(t *testing.T) {
		c := newCommitted(t)
		ctx := testContext(t)
		p := c.character("Ada")
		holder, commit := c.hold(ctx, `insert into conversations (user_id, quote_id, persona_id) values ($1, $2, $3)`,
			c.user, c.quote.ID, p.ID)
		rename := usecase.NewUpdateCharacter(c.txm, c.repo)
		done := c.whileBlocked(ctx, holder, func() error {
			_, err := rename.Execute(ctx, c.user, p.ID, domain.CharacterPatch{Name: ptr("Augusta")})
			return err
		})
		if err := commit(); err != nil {
			t.Fatal(err)
		}
		if err := <-done; !errors.Is(err, domain.ErrNameLocked) {
			t.Errorf("rename = %v, want ErrNameLocked", err)
		}
		if got, err := c.repo.GetOwn(ctx, c.user, p.ID); err != nil || got.Name != "Ada" {
			t.Errorf("character = %#v, %v; want the name unchanged", got, err)
		}
	})
	t.Run("a direct rename meets the trigger", func(t *testing.T) {
		c := newCommitted(t)
		ctx := testContext(t)
		p := c.character("Ada")
		holder, commit := c.hold(ctx, `insert into conversations (user_id, quote_id, persona_id) values ($1, $2, $3)`,
			c.user, c.quote.ID, p.ID)
		done := c.whileBlocked(ctx, holder, func() error {
			_, err := c.pool.Exec(ctx, `update personas set name = 'Augusta' where id = $1`, p.ID)
			return err
		})
		if err := commit(); err != nil {
			t.Fatal(err)
		}
		if err := <-done; !kernelpg.IsCheckViolationOf(err, "personas_name_locked") {
			t.Errorf("rename = %v, want personas_name_locked", err)
		}
	})
	t.Run("a rename first makes the discussion wait, then both stand", func(t *testing.T) {
		c := newCommitted(t)
		ctx := testContext(t)
		p := c.character("Ada")
		holder, commit := c.hold(ctx, `update personas set name = 'Augusta' where id = $1`, p.ID)
		var convID string
		done := c.whileBlocked(ctx, holder, func() error {
			var err error
			convID, err = c.convs.Create(ctx, c.user, c.quote.ID, p.ID)
			return err
		})
		if err := commit(); err != nil {
			t.Fatal(err)
		}
		if err := <-done; err != nil || convID == "" {
			t.Fatalf("discussion = %q, %v", convID, err)
		}
		got, err := c.repo.GetOwn(ctx, c.user, p.ID)
		if err != nil || got.Name != "Augusta" || !got.NameLocked() {
			t.Errorf("character = %#v, %v; want renamed, then locked", got, err)
		}
	})
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

const capRounds = 25

// R-CAP: at the cap, concurrent creates and unarchives serialize on the
// owner, so exactly one gets the last slot.
func TestConcurrentCharactersAtTheCap(t *testing.T) {
	c := newCommitted(t)
	ctx := testContext(t)
	const limit domain.CharacterLimit = 1
	create := usecase.NewCreateCharacter(c.txm, c.repo, limit)
	unarchive := usecase.NewUnarchiveCharacter(c.txm, c.repo, limit)
	reset := func(t *testing.T) {
		t.Helper()
		if _, err := c.db.Exec(ctx, `delete from personas where owner_id = $1`, c.user); err != nil {
			t.Fatal(err)
		}
	}
	active := func(t *testing.T) int {
		t.Helper()
		n, err := c.repo.CountActive(ctx, c.user)
		if err != nil {
			t.Fatal(err)
		}
		return n
	}
	check := func(t *testing.T, round int, errs []error) {
		t.Helper()
		ok, refused := 0, 0
		for _, err := range errs {
			switch {
			case err == nil:
				ok++
			case errors.Is(err, domain.ErrCharacterLimit):
				refused++
			default:
				t.Fatalf("round %d: %v", round, err)
			}
		}
		if ok != 1 || refused != len(errs)-1 || active(t) != 1 {
			t.Errorf("round %d: %d succeeded, %d refused, %d active; want exactly 1 active", round, ok, refused, active(t))
		}
	}
	newOne := func(name string) func() error {
		return func() error {
			_, err := create.Execute(ctx, c.user, domain.CharacterDraft{Name: name, Prompt: "You are " + name + "."})
			return err
		}
	}

	t.Run("creates", func(t *testing.T) {
		for round := range capRounds {
			reset(t)
			check(t, round, together(newOne("Ada"), newOne("Bo"), newOne("Cy")))
		}
	})
	t.Run("a create and an unarchive", func(t *testing.T) {
		for round := range capRounds {
			reset(t)
			p := c.character("Archived")
			if _, err := c.db.Exec(ctx, `update personas set archived_at = now() where id = $1`, p.ID); err != nil {
				t.Fatal(err)
			}
			check(t, round, together(newOne("Ada"), func() error {
				_, err := unarchive.Execute(ctx, c.user, p.ID)
				return err
			}))
		}
	})
	reset(t)
}

// The owner lock really serializes: a second create waits while the first
// holds the profile row.
func TestCreateWaitsForTheOwnerLock(t *testing.T) {
	c := newCommitted(t)
	ctx := testContext(t)
	holder, commit := c.hold(ctx, `select 1 from profiles where id = $1 for no key update`, c.user)
	create := usecase.NewCreateCharacter(c.txm, c.repo, 5)
	done := c.whileBlocked(ctx, holder, func() error {
		_, err := create.Execute(ctx, c.user, domain.CharacterDraft{Name: "Ada", Prompt: "You are Ada."})
		return err
	})
	if err := commit(); err != nil {
		t.Fatal(err)
	}
	if err := <-done; err != nil {
		t.Errorf("create after the lock was released = %v", err)
	}
}
