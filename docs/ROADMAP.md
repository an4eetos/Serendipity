# Serendipity — roadmap

Phase 0, the core loop described in [PRODUCT.md](PRODUCT.md), is built. The
roadmap features are grouped below into phases by what each one depends on.
A phase is done when its exit criteria hold. Features inside a phase can be
built in parallel unless an order is given.

## At a glance

| Phase | Theme | Features | Depends on |
|---|---|---|---|
| 0 ✓ | Core loop | Upload / Kindle import, quote, react, discuss, publish, profile | — |
| 1 | Open to strangers | Terms and ownership confirmation · report and takedown · public-coverage cap per book · reading trail | — |
| 2 | Characters | Custom characters · Socratic check | Phase 1: takedown, reading trail |
| 3 | Social | Follows · feed · same-passage reactions · comments | Phase 1: coverage cap, takedown |
| 4 | More ways in | Readwise, Apple Books and Kobo imports · page-photo OCR · public-domain catalog | — (can run alongside 2 and 3) |

Changes from the order listed in PRODUCT.md:

- **The public-coverage cap moves up to Phase 1**, and the "before launch" items
  (terms, takedown) become features. Every reader who publishes adds to how much
  of a book is public, so the cap has to exist before there are many readers.
  The takedown flow is also what custom characters and comments will use for
  moderation.
- **Custom characters move from first to Phase 2.** Discussions with them
  appear on public profiles, so a persona written by a reader needs the
  report-and-takedown path to exist before it ships.

Sizes (S / M / L) are rough estimates. Each feature is meant to be one devflow
task. Social is the exception: it splits into its four parts.

## Phase 1 — Open to strangers

Goal: invite people we don't know without legal exposure, with a public
profile that shows what someone read, not just the quotes they kept.

### Terms and ownership confirmation — S

- The reader accepts the terms of service on first sign-in
  (`profiles.terms_accepted_at`). The API refuses writes until they do.
- Uploading a book or importing Kindle clippings asks the reader to confirm
  they own the copy. The confirmation is recorded on the library item
  (`library_items.ownership_confirmed_at`).
- Contexts: profile, library, kindleimport. Web: sign-in, upload and import
  dialogs.
- Not engineering work, but this phase can't finish without it: legal review
  of the terms, the takedown process and, for Phase 4, what counts as public
  domain where.

### Report and takedown — M

- A signed-in reader can report a public quote or discussion. Rights holders
  without an account get a DMCA contact page.
- An admin can hide reported content. Hidden content disappears from profiles
  and quote pages, and the owner sees why it was hidden.
- New `moderation` context with `reports` and `takedowns` tables. A takedown
  sets `removed_at` on the quote or conversation through those contexts' ports
  (moderation → quote, discussion). Public reads only add
  `removed_at is null`, and the context graph stays acyclic.
- A report targets a kind plus an id, so later reportable things (custom
  characters, comments) can be added without a schema change.
- Open: how admins are identified, either Supabase `app_metadata.role` or an
  allowlist of user ids in config.

### Public-coverage cap per book — M

- Limits how much of one work can be public across all users, so public quotes
  can't be put back together into the book.
- What counts is distinct public passages per work. Identical text (normalized,
  then hashed) counts once, so a famous line highlighted by 200 readers doesn't
  use up the cap. Selections that overlap but differ count twice, which errs on
  the safe side.
- Checked in `UpdateQuote` when a quote is made public, inside the transaction,
  with a lock on the work row so two publishes can't both get under the cap.
  A trigger enforces it in the database too, like the other publishing
  guardrails.
- The cap is a word count per work, configurable, with a per-work override in
  catalog (Phase 4 exempts public-domain works).
- Open: the cap value.
- Open: what the reader sees when the cap is reached. Recommended: publish the
  reaction and collapse the passage to its citation, instead of refusing to
  publish. The reaction is the point of the product, and it stays public.

### Reading trail — S

- A strip from 0 to 100 % on the book page and the public profile, showing
  where in the book each quote came from.
- Already in place: both readers store `quotes.position` (EPUB percentage, PDF
  page ÷ pages), and the public profile already returns it
  (`ThreadResponse.position`). This is mostly web work.
- Kindle quotes have no position: the clippings file gives locations but not
  the book's length. List them as unplaced. Estimating from the highest
  location in the file would spread a half-read book across the whole strip.

### Phase 1 exit criteria

- Legal review is signed off and the terms are live.
- An admin can hide a reported public quote, and it disappears from every
  public page.
- Both the API and the database enforce the coverage cap, and a test with
  concurrent publishes shows the cap can't be exceeded.
- Public profiles show a reading trail for each book.

## Phase 2 — Characters

Goal: discussions go further. Readers get characters of their own, and
Socrates can examine them on a book they finished.

### Custom characters — M

- Readers write their own persona: name, era, short bio and prompt (voice and
  worldview). The shared rules in `discussion/domain/prompt.go` are appended
  just as they are for presets, so the brevity and copyright rules still apply.
- Already in place: `personas.owner_id`, the "personas: own" policy, and
  `PersonaRepository.List` / `Get`, which already return the reader's own
  characters. Missing: create, edit and archive usecases and endpoints, plus
  the editor in the web app.
- No living people: a rule in the terms, backed by the report path, because
  it can't be checked automatically. The database already caps prompts at
  8,000 characters.
- On a public quote, discussions with a custom character show its name and
  bio. The prompt stays private. Custom characters can be reported (Phase 1).
- Open: deleting a character that has conversations
  (`conversations.persona_id` is `on delete restrict`). Recommended: archive
  it, so public discussions keep their character.

### Socratic check — L

- Once a book is marked finished, the reader can ask Socrates to examine them
  on it. If their answers hold up, the book gets a *verified read* badge on
  their profile.
- Base the questions on the reader's own quotes and reactions from across the
  book (the Phase 1 trail), not on what the model remembers of the book. This
  works for obscure books, keeps the "don't reproduce the source / don't
  invent facts" rules intact, and tests the reader's own engagement rather
  than plot trivia.
- To be eligible, the library item must be `finished` and have at least N
  quotes spread across the book, for example in three different fifths.
  Open: N and the spread.
- At the end, the model returns a structured verdict ("pass" or "not yet",
  with a reason). It is stored with the transcript, and a reader can try again
  after a cooldown. The badge can be gamed, so its wording should say Socrates
  was satisfied rather than claim proof.
- New `readingcheck` context: library, quote, discussion ← readingcheck ←
  profile. It needs its own table, because `conversations.quote_id` is
  `not null` and an examination isn't about a single quote. It reuses
  `CharacterModel` and the SSE stream.

### Phase 2 exit criteria

- A reader can create, edit and archive a character and discuss a quote with
  it, and a prompt test shows the shared rules are applied.
- A finished book with enough quotes spread across it can be examined, and a
  pass shows a badge on the public profile.

## Phase 3 — Social

Goal: readers find each other through what they think about the same books.

Build in this order: follows → feed → same-passage reactions → comments.
Comments come last because they're the only feature where strangers write on
someone else's content.

- **Follows — S.** Follow or unfollow a public profile. Private profiles can't
  be followed.
- **Feed — M.** Newly published reactions from the people you follow, newest
  first. Needs `quotes.published_at`, because a quote becomes public long after
  `created_at`. Uses cursor pagination on (`published_at`, `id`) with an index
  to match, since the SQL rules ban deep offsets.
- **Same-passage reactions — M.** A public quote page shows other readers'
  public reactions to the same passage. Open: what "same passage" means across
  editions, since CFIs and page numbers differ from file to file. Start with
  the exact normalized-text match the coverage cap already uses, then consider
  matching overlaps.
- **Comments — M.** Comments on public reactions. The quote's owner can delete
  comments on it, comments can be reported (Phase 1), and posting is
  rate-limited.
- New `social` context. Fix its direction relative to profile before starting:
  social imports profile (the feed shows profiles), so follower counts on the
  profile page come from a social endpoint or are composed in the web app.
  Profile never imports social.

### Phase 3 exit criteria

- A reader who follows others gets a paginated feed.
- A public quote page shows other readers' reactions to the same passage.
- An admin can hide a reported comment.

## Phase 4 — More ways in

Goal: fewer readers stuck at "bring your book". This phase doesn't depend on
Phases 2 or 3 and can run alongside them if there's capacity.

- **More imports — M each.** Readwise comes first, because one integration
  covers readers who already sync Apple Books, Kobo and other apps into it.
  Apple Books and Kobo exports follow for readers who don't use Readwise. Each
  source adds a `quote_source` value. When the second source lands, turn
  `kindleimport` into an `import` context with one parser per source. They all
  share the existing usecase: catalog the work, add the library item and
  de-duplicate quotes, all in one transaction. Each parser gets tests built
  from a real export sample, as `kindleimport/domain/testdata` does today.
- **Page-photo OCR — M.** Photograph a page of a physical book, select the
  passage from the recognized text, and type the page number as the citation.
  Recognition runs on Claude vision through the Anthropic SDK the backend
  already uses. The photo isn't stored, only the selected quote, which keeps
  the 300-word limit. Adds a `photo` source.
- **Public-domain catalog — M/L.** Standard Ebooks and Project Gutenberg books
  that everyone can read in the app. This changes the copyright model: works
  get a public-domain flag, their files are public, and the coverage cap
  doesn't apply to them. A book in the US public domain may still be under
  copyright elsewhere, so this depends on the Phase 1 legal review. Worth
  moving earlier if readers turn out to drop off between sign-up and their
  first quote, because it removes the need to own a file.

### Phase 4 exit criteria

- Each import source turns a real export into books and quotes, and
  re-importing the same file adds nothing.
- A photographed page becomes a cited quote, and the photo is not kept.
- A public-domain book can be read and quoted by any signed-in reader.
