# Serendipity — product spec

## Idea

A reading platform for people who actually read. You highlight passages from
**your own copy** of a book, bring them to a character — Socrates, Diogenes,
Seneca, Marcus Aurelius, Nietzsche, Montaigne — and get their **reaction**, or
**argue** with them. Your quotes, your notes and those conversations live on a
profile that shows what you read and what you think about it: your perspective
next to the author's.

## Core loop

1. **Bring your book** — upload an EPUB/PDF (private to you) or import Kindle
   highlights (`My Clippings.txt`).
2. **Quote** — select a passage in the reader (≤ 300 words). It's saved with a
   citation (chapter/percent or page) and stays private.
3. **Talk** — pick a character and a mode:
   - *React*: the character responds in their own voice.
   - *Argue*: the character pushes back on the passage or on your note.
   You can keep replying; several characters can weigh in on the same quote.
4. **Publish** — add your own note and publish. The quote, note and its
   conversations appear on your profile.
5. **Profile** — public or private. Shows categories (from book subjects),
   bookshelf with reading progress, and public quotes with conversations.

## Copyright model

The platform hosts catalog metadata, never the books themselves.

| Content | Where it lives | Who can see it |
|---|---|---|
| Uploaded EPUB/PDF | Private storage bucket, path `<user id>/…` | Only the uploader (short-lived signed URLs) |
| Book metadata | `works` table (Open Library) | Everyone |
| Quote | `quotes` table | Owner; public only when published |
| Conversation | `conversations`/`messages` | Follows the quote's visibility |

Guardrails enforced today:

- Quotes are capped at 300 words (API) and 3,000 characters (database).
- A quote can only be public with the reader's own note **and** a citation
  (API check + database constraint) — commentary, not copying.
- Characters are instructed never to quote more than a sentence of the source
  and never to reproduce books from memory.
- Kindle imports land private; re-imports are de-duplicated.

Before launch: terms of service (uploaders confirm they own their copy), a
DMCA/takedown process, and legal review. This document is not legal advice.

## Characters

Presets are historical figures only (no living people). Each persona prompt
holds only voice and worldview; shared rules (react, don't summarize; stay
brief; reply in the reader's language; don't reproduce the source; don't
invent facts about unknown books) are added by the API so they apply to every
character, including future custom ones.

Model: `claude-opus-5-5` at low effort, streaming, with server-side refusal
fallbacks (`fallbacks: "default"`). Configurable via `ANTHROPIC_MODEL`.

## Roadmap

Next, roughly in order:

- **Custom characters** — readers write their own persona prompt (schema already
  supports `personas.owner_id`).
- **Reading trail** — map of where in a book your quotes come from
  (`quotes.position` is already stored) as proof of reading.
- **Socratic check** — Socrates questions you on a book you finished; passing
  earns a *verified read* badge.
- **Public-coverage cap per book** — limit how much of one copyrighted work can
  be public across all users, so aggregation can't reconstruct it.
- **Social** — follows, comments on public quotes, a feed of fresh reactions.
- **Public-domain catalog** — Standard Ebooks / Project Gutenberg books readable
  by everyone.
- **Page-photo OCR** — quote from a physical book by photographing the page.
- More imports (Apple Books, Kobo, Readwise).
