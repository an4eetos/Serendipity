// Renders a public profile. Used by the server page and by the owner's
// client-side preview of a private profile, so it must not use hooks.
import type { PublicProfile } from "@/lib/types";

const statusLabel = { reading: "Reading", finished: "Finished", abandoned: "Set aside" } as const;

export function ProfileView({ data, preview = false }: { data: PublicProfile; preview?: boolean }) {
  const { profile, categories, books, quotes } = data;
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

      <section className="space-y-6">
        <h2 className="text-sm font-medium text-muted">Quotes</h2>
        {quotes.length === 0 && <p className="text-muted">No public quotes yet.</p>}
        {quotes.map((q) => (
          <article key={q.id} className="card space-y-4 p-5">
            <blockquote className="border-l-2 border-accent pl-4">
              <p className="quote-text whitespace-pre-wrap">{q.text}</p>
              <p className="mt-2 text-sm text-muted">
                — <span className="italic">{q.work?.title}</span>
                {q.work?.authors.length ? `, ${q.work.authors.join(", ")}` : ""}
                {q.location && ` · ${q.location}`}
              </p>
            </blockquote>
            <div>
              <p className="text-xs font-medium text-muted">{profile.display_name || profile.username} wrote</p>
              <p className="mt-1 whitespace-pre-wrap">{q.note}</p>
            </div>
            {q.conversations.map((c) => (
              <details key={c.id} className="rounded-md border border-line px-3 py-2">
                <summary className="cursor-pointer text-sm">
                  {c.mode === "argue" ? "Argued with" : "Reaction from"} <strong>{c.persona.name}</strong>
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
                </div>
              </details>
            ))}
          </article>
        ))}
      </section>
    </div>
  );
}
