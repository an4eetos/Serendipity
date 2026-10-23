# Serendipity — product spec

## Idea

A reading platform for people who actually read. You highlight passages from
**your own copy** of a book and write **your reaction** to them. That reaction
is the point: your perspective next to the author's.

If you want to go further, you can **discuss your reaction** with a character —
Socrates, Diogenes, Seneca, Marcus Aurelius, Nietzsche, Montaigne. The
character doesn't give its own take on the passage; it talks with you about
what *you* wrote: questions it, builds on it, pushes back where it would.

Your quotes, reactions and discussions live on a profile that shows what you
read and what you think about it.

## Core loop

1. **Bring your book** — upload an EPUB/PDF (private to you) or import Kindle
   highlights (`My Clippings.txt`).
2. **Quote** — select a passage in the reader (≤ 300 words). It's saved with a
   citation (chapter/percent or page) and stays private.
3. **React** — write your reaction, right when you quote or later. Kindle
   notes come in as reactions to their highlights.
4. **Discuss (optional)** — once you have a reaction, pick a character and talk
   it over. The character's first message answers your reaction; you keep
   replying. Several characters can discuss the same quote.
5. **Publish** — a quote with your reaction can be published. The quote, your
   reaction and its discussions appear on your profile.
6. **Profile** — public or private. Shows categories (from book subjects),
   bookshelf with reading progress, and public quotes with reactions and
   discussions.

## Copyright model

The platform hosts catalog metadata, never the books themselves.

| Content | Where it lives | Who can see it |
|---|---|---|
| Uploaded EPUB/PDF | Private storage bucket, path `<user id>/…` | Only the uploader (short-lived signed URLs) |
| Book metadata | `works` table (Open Library) | Everyone |
| Quote | `quotes` table | Owner; public only when published |
| Reaction | `quotes.reaction` | Follows the quote |
| Discussion | `conversations`/`messages` | Follows the quote |

Guardrails enforced today:

- Quotes are capped at 300 words (API) and 3,000 characters (database).
- A quote can only be public with the reader's own reaction **and** a
  citation (API check + database constraint) — commentary, not copying.
- A discussion can only start once the reader has written a reaction.
- Characters are instructed never to quote more than a sentence of the source
  and never to reproduce books from memory.
- Kindle imports land private; re-imports are de-duplicated.

Before launch: terms of service (uploaders confirm they own their copy), a
DMCA/takedown process, and legal review (roadmap Phase 1). This document is
not legal advice.

## Characters

Presets are historical figures only (no living people). Each persona prompt
holds only voice and worldview; shared rules (discuss the reader's reaction
rather than give a standalone take; don't summarize; stay brief; reply in the
reader's language; don't reproduce the source; don't invent facts about unknown
books) are added by the API so they apply to every character, including future
custom ones.

Model: `claude-opus-5-5` at low effort, streaming, with server-side refusal
fallbacks (`fallbacks: "default"`). Configurable via `ANTHROPIC_MODEL`.

## Roadmap

Phased, with dependencies, open questions and exit criteria, in
[ROADMAP.md](ROADMAP.md):

1. **Open to strangers** — terms and ownership confirmation, report and
   takedown, public-coverage cap per book, reading trail.
2. **Characters** — custom characters, Socratic check (*verified read* badge).
3. **Social** — follows, feed, same-passage reactions, comments.
4. **More ways in** — Readwise / Apple Books / Kobo imports, page-photo OCR,
   public-domain catalog.
