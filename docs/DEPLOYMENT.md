# Serendipity — deployment plan (Railway + Supabase)

Production runs as two Railway services backed by one hosted Supabase project.

```
browser ──► web  (Railway, Next.js, root /web)   ──► api (server-side fetch: /terms, /profiles)
   │                                                  │
   ├──► api  (Railway, Go, root /backend) ◄───────────┘
   │        ├──► Supabase Postgres  (session pooler, IPv4)
   │        ├──► Supabase Storage   (signed URLs, service key)
   │        ├──► Supabase Auth JWKS (token verification)
   │        ├──► Anthropic API
   │        └──► Open Library
   └──► Supabase Auth (magic links) + Storage (uploads into `books`)
```

## 0. Decisions to make first

| Decision | Recommendation |
|---|---|
| Region | Put Supabase and Railway in the same region (e.g. both `us-east` or both `eu-west`). Every API request makes several DB round trips. |
| Supabase plan | Pro once real readers arrive. Free projects pause after a week of inactivity and have no backups. Free is fine for a private dry run. |
| Email (SMTP) | Resend or Postmark. Supabase's built-in SMTP is rate-limited and only delivers to team members, so magic-link sign-in fails for everyone else. |
| Domains | Start with Railway's `*.up.railway.app` domains, then add a custom domain (e.g. `serendipity.app` → web, `api.serendipity.app` → api). |
| Staging | One production environment to begin with. Add a second Supabase project + Railway environment before Phase 2. |
| Launch gate | The terms page says "Draft — pending legal review". Per the roadmap, Phase 1 isn't done until legal review is finished. Deploying is fine; inviting strangers waits for it. |

## 1. Repo changes for the deploy

Backend and database (done):

- [`backend/Dockerfile`](../backend/Dockerfile): builds on `golang:1.27-alpine` and ships a static binary on `distroless/static:nonroot` (~32 MB). [`backend/.dockerignore`](../backend/.dockerignore) keeps `.env` out of the build context.
- [`backend/railway.toml`](../backend/railway.toml): Dockerfile builder, `/backend/**` watch paths, `/healthz` health check, restart on failure. Railway doesn't resolve the config file against the root directory, so set **Config File Path** to `/backend/railway.toml` in the service settings.
- `infra/config`: the listen address is `API_ADDR` if set, otherwise `:$PORT` (set by Railway), otherwise `:8787`. Leave `API_ADDR` **unset** on Railway.
- [`supabase/seed.sql`](../supabase/seed.sql): `on conflict (slug) do nothing`, so running the seed again is harmless.

Verified locally: `go test ./...` passes. The image starts on `$PORT` against local Supabase and serves `/healthz` and `/terms`. It applies CORS for `ALLOWED_ORIGINS` and exits 0 on SIGTERM. It exits 1 when the database is unreachable, which fails the Railway health check and keeps the previous deploy serving. The seed runs twice without error and leaves 6 presets.

Web (still to do):

- `web/package.json`: add `"engines": { "node": ">=22" }` so the builder uses Node 22.
- `web/railway.toml` (optional): `watchPatterns = ["/web/**"]`, `healthcheckPath = "/"`.

## 2. Supabase project

1. Create the project in the chosen region and save the DB password in a password manager.
2. **Apply the schema and seed** from your machine:

   ```sh
   npx supabase login
   npx supabase link --project-ref <ref>
   npx supabase db push --dry-run          # should list the 2 migrations
   npx supabase db push --include-seed     # migrations + preset characters
   ```

   Running the seed again skips presets that already exist. Check that `select slug from public.personas where is_preset;` returns 6 rows and that the `books` bucket exists under Storage.
3. **Auth → URL configuration**
   - Site URL: `https://<web domain>`
   - Redirect URLs: `https://<web domain>/**`. Add the custom domain here too when you get one.
4. **Auth → SMTP**: enter the Resend/Postmark credentials, set a sender like `no-reply@<domain>`, and verify the domain's SPF/DKIM. Raise the email rate limit above its default of 2/hour.
5. **Auth → Email templates**: customise the magic-link email. Leave signups enabled.
6. **Keys** (Project settings → API keys):
   - Web: the anon/publishable key.
   - API: the **legacy `service_role` key (JWT)**. `library/adapters/supabase/storage.go` sends the key as both `apikey` and `Authorization: Bearer`. A new-format `sb_secret_…` key isn't a JWT and may be rejected in the Bearer header. Either use the legacy key, or check `/library/{id}/file-url` with the new key during the smoke test (§5).
   - JWT: new projects sign access tokens with asymmetric keys, and the API verifies those through JWKS (`/auth/v1/.well-known/jwks.json`), so you can leave `SUPABASE_JWT_SECRET` empty. Set it only if the project still uses the legacy HS256 secret.
7. **Database → Connection string → Session pooler** (IPv4). Don't use the direct `db.<ref>.supabase.co` host: it's IPv6-only unless you buy the IPv4 add-on, and Railway's egress is IPv4. Don't use the transaction pooler on port 6543 either, because pgx's prepared-statement cache breaks behind it. The connection string:

   ```
   postgresql://postgres.<ref>:<url-encoded password>@aws-0-<region>.pooler.supabase.com:5432/postgres?sslmode=require&pool_max_conns=5
   ```

   `pool_max_conns=5` keeps one API replica well inside the pooler's client limit on small plans.

## 3. Railway project

Create one project from the GitHub repo with two services. Generate both public domains **before** setting variables, so the services can reference each other's domain.

### `api` service

- Settings: root directory `/backend`, config file path `/backend/railway.toml`, 1 replica. The build, health check and restart policy come from the config file.
- If the DB is unreachable at startup, the deploy fails its health check and the previous deploy keeps serving. That's the intended behavior: `database.NewPool` pings on start.

| Variable | Value |
|---|---|
| `PORT` | `8787`: pinned so the public domain and `web`'s private `API_URL` target a known port. Leave `API_ADDR` unset; the API listens on `$PORT`. |
| `DATABASE_URL` | session-pooler string from §2.7 |
| `SUPABASE_URL` | `https://<ref>.supabase.co` |
| `SUPABASE_SERVICE_ROLE_KEY` | service key from §2.6 |
| `SUPABASE_JWT_SECRET` | empty, unless the project uses legacy HS256 |
| `ANTHROPIC_API_KEY` | production key, ideally from its own workspace with a spend limit |
| `ANTHROPIC_MODEL` | `claude-opus-5-5` (the default; set it explicitly so it's visible) |
| `ALLOWED_ORIGINS` | `https://${{web.RAILWAY_PUBLIC_DOMAIN}}`, plus the custom domain later, comma-separated |
| `TERMS_VERSION` | the version of the terms you publish (e.g. `2026-11-01`) |
| `PUBLIC_WORD_CAP` | `2500` |

### `web` service

- Settings: root directory `/web`, default Railpack builder (`npm ci && npm run build`, start `npm run start`; `next start` reads `$PORT`).
- Next.js inlines `NEXT_PUBLIC_*` at **build** time, so changing one requires a redeploy. Railway exposes service variables at build time, so setting them on the service is enough.

| Variable | Value |
|---|---|
| `NEXT_PUBLIC_SUPABASE_URL` | `https://<ref>.supabase.co` |
| `NEXT_PUBLIC_SUPABASE_ANON_KEY` | anon/publishable key |
| `NEXT_PUBLIC_API_URL` | `https://${{api.RAILWAY_PUBLIC_DOMAIN}}` |
| `API_URL` | `http://${{api.RAILWAY_PRIVATE_DOMAIN}}:8787`: used by the server-rendered `/terms` and `/u/[username]` pages over private networking. These fetches happen at request time, so the build doesn't need the API. |
| `NEXT_PUBLIC_TAKEDOWN_EMAIL` | the DMCA / takedown address shown on `/takedown` |

## 4. First deploy order

1. Merge the repo changes from §1.
2. Set up Supabase (§2).
3. Deploy `api` and check that `https://<api>/healthz` returns `ok` and `https://<api>/terms` returns the version.
4. Deploy `web`.
5. Point Supabase Auth's Site URL and redirect URLs at the web domain (§2.3).

## 5. Smoke test

- [ ] Magic-link sign-in from a non-team email address (tests SMTP, the redirect URL and the JWKS check)
- [ ] First sign-in asks you to accept the terms; a write before accepting returns 403
- [ ] Open Library search, add a book, upload an EPUB/PDF with ownership confirmed, reopen it (tests the signed URL and the service key)
- [ ] Kindle import of a real `My Clippings.txt`
- [ ] Create a quote with a reaction and start a conversation with Socrates. The reply should **stream** in chunks rather than arrive all at once (SSE through the Railway proxy).
- [ ] Make the profile public and open `/u/<username>` in a private window (server-side fetch over the private network)
- [ ] Insert yourself as admin (`insert into public.admins (user_id) values ('<uuid>');` in the SQL editor), report a quote from a second account, then take it down and restore it
- [ ] Browser console shows no CORS errors

## 6. Running it

- **Schema changes**: add a migration under `supabase/migrations/`, run `npx supabase db push --dry-run` and then `db push` against production **before** merging code that needs the change. Migrations are forward-only, so take a backup first (Pro: Database → Backups).
- **Deploys**: pushing to `main` rebuilds only the service whose watch paths changed. Turn on Railway's "Wait for CI" with a GitHub Action that runs `go test ./...` and the web lint/typecheck, so a red build never ships.
- **Rollback**: redeploy the previous build from Railway's deployments list. A database rollback means writing a new migration.
- **Config bumps**: changing `TERMS_VERSION` on `api` and redeploying makes every reader accept the terms again.
- **Watch**: Railway logs (chi's request log, `slog` errors), the Supabase database CPU and connection count, and Anthropic spend.
- **Secrets**: rotate the service key or Anthropic key by updating the Railway variable, which redeploys the service. Never commit `backend/.env` or `web/.env.local`.

## 7. Known risks

| Risk | Mitigation |
|---|---|
| A new-format Supabase secret key is rejected as a Bearer token by Storage | Use the legacy `service_role` key, or drop the `Authorization` header in `storage.go` (a separate change). |
| Supabase's default SMTP blocks sign-in for outside users | Custom SMTP (§2.4) before inviting anyone. |
| Pooler client limit exhausted | `pool_max_conns` in `DATABASE_URL`; budget it per replica. |
| JWKS fetch fails at boot | The API logs "JWKS unavailable" and rejects asymmetric tokens. Redeploy, and alert on that log line. |
| Free Supabase project pauses | Upgrade to Pro before launch. |
| Terms are still a draft | Legal review before opening to strangers (roadmap, Phase 1). |
