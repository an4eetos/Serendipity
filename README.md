# Serendipity

Read it for real, then say something new. Quote passages from your own copy of
a book and write your reaction to them. If you want, talk your reaction over
with Socrates, Diogenes or another character. Your quotes, reactions and
discussions live on a profile that shows what you actually read and what you
think about it.

Product spec and copyright model: [docs/PRODUCT.md](docs/PRODUCT.md).

## Layout

```
backend/    Go API — chi, pgx, Uber Fx, Anthropic Go SDK
web/        Next.js (App Router, TypeScript, Tailwind)
supabase/   local Supabase config, migrations, seed (preset characters)
docs/       product spec
```

The backend is organized by business capability (rules in
[.claude/rules/go](.claude/rules/go)):

```
backend/
  cmd/server/     main: bootstraps the Fx app
  infra/          config, database, HTTP server; one Fx module per bounded context
  internal/
    kernel/       shared kernel: DomainError, TxManager, Postgres + HTTP plumbing
    identity/     Supabase access tokens, auth middleware
    catalog/      works and Open Library search
    library/      a reader's books, progress, private files
    quote/        quotes and the publishing rules
    discussion/   characters, conversations, quote pages
    kindleimport/ My Clippings.txt import
    profile/      profiles and the public profile page
```

Each bounded context has `domain/` (pure rules, standard library only),
`ports/` (interfaces), `usecase/` (one business action each, writes inside
the TxManager) and `adapters/` (`handlers` and `converters` for HTTP;
`postgres`, `openlibrary`, `supabase`, `anthropic` for the outside world).

The web app talks to Supabase only for sign-in and for uploading books into the
private `books` bucket; everything else goes through the Go API.

## Running locally

Requirements: Docker, Go 1.27+, Node 22+.

```sh
# 1. Supabase (Postgres, Auth, Storage) — applies migrations + seed
npx supabase start
npx supabase status -o env   # local URLs and keys

# 2. API on :8787
cd backend
cp .env.example .env         # fill SUPABASE_SERVICE_ROLE_KEY, SUPABASE_JWT_SECRET, ANTHROPIC_API_KEY
set -a; . ./.env; set +a
go run ./cmd/server

# 3. Web on :3000
cd web
cp .env.example .env.local   # fill NEXT_PUBLIC_SUPABASE_ANON_KEY
npm install
npm run dev
```

Sign-in uses email magic links; locally the emails land in Mailpit at
http://127.0.0.1:54324.

Reset the database (re-run migrations and seed): `npx supabase db reset`.

## Tests

```sh
cd backend && go test ./...  # domain rules, usecases with fake ports, adapters
cd web && npm run lint && npx tsc --noEmit
```

The Postgres adapter tests run against the local Supabase database and are
skipped unless `TEST_DATABASE_URL` is set. Each test runs in a transaction that
is rolled back, so it leaves no data behind:

```sh
cd backend && TEST_DATABASE_URL=postgresql://postgres:postgres@127.0.0.1:54322/postgres go test ./...
```

## API

All endpoints except `/healthz` and `/profiles/{username}` need a Supabase
access token (`Authorization: Bearer …`).

| Method | Path | |
|---|---|---|
| GET/PATCH | `/me` | Own profile (username, display name, bio, public) |
| GET | `/profiles/{username}` | Public profile: categories, books, public quotes + conversations |
| GET | `/works/search?q=` | Open Library search |
| GET/POST | `/library` | List / add a book |
| GET/PATCH | `/library/{id}` | Book, progress, uploaded file |
| GET | `/library/{id}/file-url` | Signed URL for the owner's file (10 min) |
| POST | `/imports/kindle` | Multipart `file` = My Clippings.txt |
| GET/POST | `/quotes` | List (optionally `?work_id=`) / create (with optional `reaction`) |
| GET/PATCH/DELETE | `/quotes/{id}` | Quote + discussions; edit reaction, visibility |
| GET | `/personas` | Characters |
| POST | `/conversations` | Discuss a quote's reaction with a character (needs a reaction); streams SSE |
| POST | `/conversations/{id}/messages` | Reply; streams SSE |
