//go:build e2e

// Package e2e runs the Phase 1 ("Open to strangers") and Phase 2 ("Custom
// characters") API scenario end to end
// against the local stack: the real server binary built from ./cmd/server,
// real JWTs from local Supabase Auth, real Supabase Storage and the local
// Postgres. It replaces the uncommitted script described in
// artifacts/phase1-open-to-strangers/04-e2e/added-cases.md §3.
//
// Run it from backend/ with `npx supabase start` running:
//
//	E2E=1 go test -tags e2e -count=1 -v ./e2e/...
//
// Environment:
//   - E2E=1 is required; without it the test skips.
//   - DATABASE_URL, SUPABASE_URL and SUPABASE_SERVICE_ROLE_KEY (plus whatever
//     else the server needs, such as SUPABASE_JWT_SECRET) come from
//     backend/.env; a variable set in the process environment wins. Both URLs
//     must point at 127.0.0.1 or localhost, or the test refuses to run.
//   - SUPABASE_ANON_KEY signs the users in. When it is unset, ANON_KEY is
//     read from `npx supabase status -o env` in the repository root.
//
// The server runs on a free 127.0.0.1 port with PUBLIC_WORD_CAP=12,
// CUSTOM_CHARACTER_LIMIT=2 and TERMS_VERSION=e2e-<random>. Its model client
// points at a closed local port (ANTHROPIC_BASE_URL), so no request ever
// reaches Anthropic and no key is needed. Three throwaway users (readers A
// and B, admin C) are created with tagged emails
// (e2e-<hex>-a@example.test). Only the admins row and two answered
// discussions (one with a preset, one with A's character) are inserted with
// SQL. Cleanup is
// registered before anything is created, so it also runs after a failure:
// it stops the server, deletes the storage objects, the users (which
// cascades to their data) and the tagged works, and checks that nothing
// tagged is left and that the row counts match the baseline.
package e2e

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	// publicWordCap is the server's PUBLIC_WORD_CAP: two 8-word passages
	// overflow it.
	publicWordCap = "12"
	// characterLimit is the server's CUSTOM_CHARACTER_LIMIT.
	characterLimit = "2"
	// closedModelURL is where the server's model client sends requests: a
	// closed local port, so a reply fails fast and nothing leaves the machine.
	closedModelURL = "http://127.0.0.1:9"
	httpTimeout    = 30 * time.Second
)

// reader is one throwaway user.
type reader struct {
	name     string
	id       string
	email    string
	password string
	token    string
	username string
}

// scenario is the state shared by the steps, in the order they fill it.
type scenario struct {
	tag      string
	terms    string
	tmpDir   string
	cfg      stackConfig
	sb       *supabase
	db       *pgxpool.Pool
	srv      *server
	api      *apiClient
	baseline rowCounts
	a, b, c  reader
	users    []string // ids of every created user, for cleanup
	works    []string // ids of every created work, for cleanup
	objects  []string // storage paths that may exist, for cleanup

	itemA       string // A's library item of workA
	workA       string // the shared work of the cap steps
	kindleWork  string
	kindleQuote string
	objectPath  string
	q1, q2, q3  string // A's shown and collapsed quotes; B's identical passage
	conv        string // the answered discussion of q1
	takedown    string

	// Phase 2: A's characters, the discussion with char1, and the takedowns
	// of char1 and of that discussion.
	charPrompt     string
	char1, char2   string
	char3          string
	charConv       string
	charTakedown   string
	charConvRemove string

	cleaned bool
}

// TestPhase1Scenario walks the Phase 1 API scenario: terms, ownership,
// storage, the public word cap, the public profile, reports, moderation and
// cleanup, and Phase 2's custom characters: the cap, the name lock, archive,
// reports, takedowns and the prompt's privacy. Each step is a subtest, and each check inside it is its own
// subtest, so a failure names the step.
func TestPhase1Scenario(t *testing.T) {
	if os.Getenv("E2E") != "1" {
		t.Skip("API end-to-end scenario: set E2E=1 and start the local Supabase stack " +
			"(npx supabase start) to run it; see the package doc")
	}
	s := &scenario{tag: "e2e-" + randomHex(t, 4)}
	s.terms = s.tag
	// Registered first, so it runs last and even when a step fails.
	t.Cleanup(func() { s.finalCleanup(t) })
	s.setup(t)

	steps := []struct {
		name string
		run  func(*testing.T)
	}{
		{"1_terms", s.stepTerms},
		{"2_ownership", s.stepOwnership},
		{"3_storage", s.stepStorage},
		{"4_cap", s.stepCap},
		{"5_public_profile", s.stepPublicProfile},
		{"6_reports", s.stepReports},
		{"7_admin", s.stepAdmin},
		{"8_characters", s.stepCharacters},
		{"9_character_archive", s.stepCharacterArchive},
		{"10_character_moderation", s.stepCharacterModeration},
		{"11_cleanup", s.stepCleanup},
	}
	for _, st := range steps {
		t.Run(st.name, st.run)
	}
}

// setup loads the stack, records the baseline, starts the server and
// creates and signs in the three users.
func (s *scenario) setup(t *testing.T) {
	s.cfg = loadStack(t)
	hc := &http.Client{Timeout: httpTimeout}
	s.sb = &supabase{url: s.cfg.supabaseURL, serviceKey: s.cfg.serviceRoleKey, anonKey: s.cfg.anonKey, hc: hc}
	var err error
	if s.db, err = pgxpool.New(t.Context(), s.cfg.databaseURL); err != nil {
		t.Fatalf("connect to the database: %v", err)
	}
	if s.baseline, err = countRows(t.Context(), s.db); err != nil {
		t.Fatalf("count baseline rows: %v", err)
	}
	if s.tmpDir, err = os.MkdirTemp("", "serendipity-e2e-"); err != nil {
		t.Fatal(err)
	}
	bin, err := buildServer(s.cfg.backendDir, s.tmpDir)
	if err != nil {
		t.Fatal(err)
	}
	env := append(s.cfg.serverEnv, "PUBLIC_WORD_CAP="+publicWordCap, "TERMS_VERSION="+s.terms,
		"CUSTOM_CHARACTER_LIMIT="+characterLimit, "ANTHROPIC_BASE_URL="+closedModelURL, "ANTHROPIC_API_KEY=e2e-unused")
	s.srv, err = startServer(bin, s.tmpDir, env)
	if err != nil {
		t.Fatalf("start the server: %v", err)
	}
	s.api = &apiClient{base: s.srv.base, hc: hc}

	s.a = s.newReader(t, "a")
	s.b = s.newReader(t, "b")
	s.c = s.newReader(t, "c")
	t.Logf("tag %s, server %s, users a=%s b=%s c=%s", s.tag, s.srv.base, s.a.id, s.b.id, s.c.id)
}

// newReader creates a tagged user, signs it in and reads its username.
func (s *scenario) newReader(t *testing.T, name string) reader {
	t.Helper()
	r := reader{name: name, email: s.tag + "-" + name + "@example.test", password: randomHex(t, 16)}
	var err error
	if r.id, err = s.sb.createUser(t.Context(), r.email, r.password); err != nil {
		t.Fatalf("user %s: %v", name, err)
	}
	s.users = append(s.users, r.id)
	if r.token, err = s.sb.signIn(t.Context(), r.email, r.password); err != nil {
		t.Fatalf("user %s: %v", name, err)
	}
	resp := s.api.call(t, http.MethodGet, "/me", r.token, nil)
	if resp.status != http.StatusOK {
		t.Fatalf("user %s: the API refused its JWT: %s", name, resp)
	}
	r.username = decode[meBody](t, resp).Username
	return r
}

// need stops a check whose input an earlier check failed to produce.
func need(t *testing.T, values ...string) {
	t.Helper()
	for _, v := range values {
		if v == "" {
			t.Fatal("skipped: an earlier check that this one depends on failed")
		}
	}
}

func randomHex(t *testing.T, n int) string {
	t.Helper()
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		t.Fatal(err)
	}
	return hex.EncodeToString(b)
}
