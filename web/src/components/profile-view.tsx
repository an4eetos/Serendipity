// Renders a public profile. Used by the server page and by the owner's
// client-side preview of a private profile, so it must not use hooks.
import type { PublicProfile } from "@/lib/types";
import { ReadingTrail } from "./reading-trail";
import { ReportButton } from "./report-button";

const statusLabel = { reading: "Reading", finished: "Finished", abandoned: "Set aside" } as const;

export function ProfileView({ data, preview = false }: { data: PublicProfile; preview?: boolean }) {
  const { profile, categories, books, quotes } = data;
  // Trails cover every public quote, but only the shown ones have an article to link to.
  const shown = new Set(quotes.map((q) => q.id));
  const trails = books.filter((b) => b.public_quotes > 0 && (b.trail.points.length > 0 || b.trail.unplaced > 0));
  return (
    <div className="space-y-10">
      <header>
        {preview && (
          <p className="mb-4 rounded-md bg-accent-soft px-3 py-2 text-sm text-accent">
            Only you can see this — your profile is private. Make it public from your library.
          </p>
        )}
        <h1 className="font-serif text-3xl font-semibold">{profile.display_name || profile.username}</h1>
        <p className="text-muted">@{profile.username}</p>
        {profile.bio && <p className="mt-2 max-w-xl">{profile.bio}</p>}
        <p className="mt-3 text-sm text-muted">
          {books.length} book{books.length === 1 ? "" : "s"} · {quotes.length} public quote
          {quotes.length === 1 ? "" : "s"}
        </p>
      </header>

      {categories.length > 0 && (
        <section>
          <h2 className="mb-2 text-sm font-medium text-muted">Reads about</h2>
          <div className="flex flex-wrap gap-1.5">
            {categories.map((c) => (
              <span key={c.name} className="rounded-full border border-line px-3 py-1 text-sm">
                {c.name}
              </span>
            ))}
          </div>
        </section>
      )}

      {books.length > 0 && (
        <section>
          <h2 className="mb-3 text-sm font-medium text-muted">Bookshelf</h2>
          <ul className="grid grid-cols-2 gap-4 sm:grid-cols-4 md:grid-cols-6">
            {books.map((b) => (
              <li key={b.work.id} className="space-y-1">
                {b.work.cover_url ? (
                  // eslint-disable-next-line @next/next/no-img-element
                  <img src={b.work.cover_url} alt="" className="aspect-[2/3] w-full rounded object-cover shadow-sm" />
                ) : (
                  <div className="flex aspect-[2/3] w-full items-center justify-center rounded bg-accent-soft p-2 text-center font-serif text-xs text-accent">
                    {b.work.title}
                  </div>
                )}
                <p className="line-clamp-2 text-sm font-medium">{b.work.title}</p>
                <p className="text-xs text-muted">
                  {b.status === "reading" ? `${Math.round(b.progress * 100)}% read` : statusLabel[b.status]}
                  {b.public_quotes > 0 && ` · ${b.public_quotes} quote${b.public_quotes === 1 ? "" : "s"}`}
                </p>
              </li>
            ))}
          </ul>
        </section>
      )}

      {trails.length > 0 && (
        <section>
          <h2 className="mb-3 text-sm font-medium text-muted">Reading trail</h2>
          <ul className="space-y-4">
            {trails.map((b) => (
              <li key={b.work.id}>
                <p className="mb-1 truncate text-sm font-medium">{b.work.title}</p>
                <ReadingTrail
                  dots={b.trail.points.map((p) => ({
                    id: p.quote_id,
                    position: p.position,
                    href: shown.has(p.quote_id) ? `#quote-${p.quote_id}` : undefined,
                  }))}
                  unplaced={b.trail.unplaced}
                />
              </li>
            ))}
          </ul>
        </section>
      )}

      <section className="space-y-6">
        <h2 className="text-sm font-medium text-muted">Quotes</h2>
        {quotes.length === 0 && <p className="text-muted">No public quotes yet.</p>}
        {quotes.map((q) => (
          <article key={q.id} id={`quote-${q.id}`} className="card scroll-mt-4 space-y-4 p-5">
            <blockquote className="border-l-2 border-accent pl-4">
              {q.passage_hidden ? (
                <p className="text-sm italic text-muted">Passage hidden: this book&apos;s public quota is full.</p>
              ) : (
                <p className="quote-text whitespace-pre-wrap">{q.text}</p>
              )}
              <p className="mt-2 text-sm text-muted">
                — <span className="italic">{q.work?.title}</span>
                {q.work?.authors.length ? `, ${q.work.authors.join(", ")}` : ""}
                {q.location && ` · ${q.location}`}
              </p>
            </blockquote>
            <div>
              <p className="text-xs font-medium text-muted">{profile.display_name || profile.username}&apos;s reaction</p>
              <p className="mt-1 whitespace-pre-wrap">{q.reaction}</p>
            </div>
            {q.conversations.map((c) => (
              <details key={c.id} className="rounded-md border border-line px-3 py-2">
                <summary className="cursor-pointer text-sm">
                  Discussed with <strong>{c.persona.name}</strong>
                  <span className="text-muted"> · {c.messages.length} message{c.messages.length === 1 ? "" : "s"}</span>
                </summary>
                <div className="mt-3 space-y-3">
                  {c.messages.map((m) => (
                    <div key={m.id}>
                      <p className="text-xs font-medium text-muted">
                        {m.role === "user" ? profile.display_name || profile.username : c.persona.name}
                      </p>
                      <p className="whitespace-pre-wrap text-[0.95rem] leading-relaxed">{m.content}</p>
                    </div>
                  ))}
                  <ReportButton targetKind="conversation" targetId={c.id} ownerId={profile.id} />
                </div>
              </details>
            ))}
            <ReportButton targetKind="quote" targetId={q.id} ownerId={profile.id} />
          </article>
        ))}
      </section>
    </div>
  );
}
