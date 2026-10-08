# Serendipity

Read it for real, then say something new. Quote passages from your own copy of
a book, get a reaction from Socrates or Diogenes (or argue with them), and keep
it all on a profile that shows what you actually read.

Product spec and copyright model: [docs/PRODUCT.md](docs/PRODUCT.md).

## Layout

```
api/        Go API — chi, pgx, Anthropic Go SDK
web/        Next.js (App Router, TypeScript, Tailwind)
supabase/   local Supabase config, migrations, seed (preset characters)
docs/       product spec
```

The web app talks to Supabase only for sign-in and for uploading books into the
private `books` bucket; everything else goes through the Go API.

## Running locally

Requirements: Docker, Go 1.27+, Node 22+.

```sh
# 1. Supabase (Postgres, Auth, Storage) — applies migrations + seed
npx supabase start
npx supabase status -o env   # local URLs and keys

# 2. API on :8787
cd api
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
cd api && go test ./...      # Kindle parser, quote rules, prompt building
cd web && npm run lint && npx tsc --noEmit
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
| GET/POST | `/quotes` | List (optionally `?work_id=`) / create |
| GET/PATCH/DELETE | `/quotes/{id}` | Quote + conversations; note, visibility |
| GET | `/personas` | Characters |
| POST | `/conversations` | Start (quote, persona, `react`/`argue`); streams SSE |
| POST | `/conversations/{id}/messages` | Reply; streams SSE |
