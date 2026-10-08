"use client";

import Link from "next/link";
import { useRef, useState } from "react";
import { api, get, patch, post } from "@/lib/api";
import { BOOKS_BUCKET, supabase } from "@/lib/supabase";
import type { FileType, KindleImportResult, LibraryItem, Profile, SearchResult } from "@/lib/types";

export function Cover({ url, title, className = "h-24 w-16" }: { url: string | null; title: string; className?: string }) {
  if (url) {
    // eslint-disable-next-line @next/next/no-img-element
    return <img src={url} alt="" className={`${className} shrink-0 rounded object-cover shadow-sm`} />;
  }
  return (
    <div
      className={`${className} flex shrink-0 items-center justify-center rounded bg-accent-soft p-1 text-center font-serif text-[0.6rem] leading-tight text-accent`}
    >
      {title.slice(0, 40)}
    </div>
  );
}

// ---------------------------------------------------------------------------

export function ProfileCard({ profile, onChange }: { profile: Profile; onChange: (p: Profile) => void }) {
  const [username, setUsername] = useState(profile.username);
  const [error, setError] = useState<string>();
  const [busy, setBusy] = useState(false);

  async function save(body: Partial<Profile>) {
    setBusy(true);
    setError(undefined);
    // Show the change right away; roll back if the API refuses it.
    if (body.is_public !== undefined) onChange({ ...profile, is_public: body.is_public });
    try {
      onChange(await patch<Profile>("/me", body));
    } catch (e) {
      onChange(profile);
      setError((e as Error).message);
    } finally {
      setBusy(false);
    }
  }

  return (
    <section className="card flex flex-wrap items-center gap-3 p-4 text-sm">
      <span className="text-muted">serendipity/u/</span>
      <input
        value={username}
        onChange={(e) => setUsername(e.target.value.toLowerCase())}
        className="input w-44 py-1"
        aria-label="Username"
      />
      {username !== profile.username && (
        <button className="btn" disabled={busy} onClick={() => save({ username })}>
          Save
        </button>
      )}
      <div className="flex-1" />
      <label className="flex cursor-pointer items-center gap-2">
        <input
          type="checkbox"
          checked={profile.is_public}
          disabled={busy}
          onChange={(e) => save({ is_public: e.target.checked })}
          className="accent-accent"
        />
        Public profile
      </label>
      <Link href={`/u/${profile.username}`} className="btn">
        View profile
      </Link>
      {error && <p className="w-full text-danger">{error}</p>}
    </section>
  );
}

// ---------------------------------------------------------------------------

export function AddBook({ onAdded }: { onAdded: (item: LibraryItem) => void }) {
  const [q, setQ] = useState("");
  const [results, setResults] = useState<SearchResult[]>();
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string>();

  async function search(e: React.FormEvent) {
    e.preventDefault();
    if (q.trim().length < 2) return;
    setBusy(true);
    setError(undefined);
    try {
      setResults(await get<SearchResult[]>(`/works/search?q=${encodeURIComponent(q)}`));
    } catch (e) {
      setError((e as Error).message);
      // Still offer adding the book by hand.
      setResults([]);
    } finally {
      setBusy(false);
    }
  }

  async function add(r: Omit<SearchResult, "first_published" | "openlibrary_key"> & { openlibrary_key: string | null }) {
    setBusy(true);
    try {
      const { openlibrary_key, title, authors, isbn, cover_url, subjects } = r;
      onAdded(await post<LibraryItem>("/library", { openlibrary_key, title, authors, isbn, cover_url, subjects }));
      setResults(undefined);
      setQ("");
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setBusy(false);
    }
  }

  return (
    <section className="card p-4">
      <h2 className="font-semibold">Add a book</h2>
      <form onSubmit={search} className="mt-3 flex gap-2">
        <input
          value={q}
          onChange={(e) => setQ(e.target.value)}
          placeholder="Title, author or ISBN"
          className="input"
        />
        <button className="btn btn-primary" disabled={busy}>
          Search
        </button>
      </form>
      {error && <p className="mt-2 text-sm text-danger">{error}</p>}
      {results && (
        <ul className="mt-3 divide-y divide-line">
          {results.map((r) => (
            <li key={r.openlibrary_key} className="flex items-center gap-3 py-2">
              <Cover url={r.cover_url} title={r.title} className="h-14 w-10" />
              <div className="min-w-0 flex-1">
                <p className="truncate font-medium">{r.title}</p>
                <p className="truncate text-sm text-muted">
                  {r.authors.join(", ")}
                  {r.first_published ? ` · ${r.first_published}` : ""}
                </p>
              </div>
              <button className="btn" disabled={busy} onClick={() => add(r)}>
                Add
              </button>
            </li>
          ))}
          <li className="py-2 text-sm text-muted">
            Not here?{" "}
            <button
              className="text-accent underline"
              disabled={busy}
              onClick={() =>
                add({ openlibrary_key: null, title: q.trim(), authors: [], isbn: null, cover_url: null, subjects: [] })
              }
            >
              Add &ldquo;{q.trim()}&rdquo; as a new book
            </button>
          </li>
        </ul>
      )}
    </section>
  );
}

// ---------------------------------------------------------------------------

export function KindleImport({ onImported }: { onImported: () => void }) {
  const input = useRef<HTMLInputElement>(null);
  const [result, setResult] = useState<KindleImportResult>();
  const [error, setError] = useState<string>();
  const [busy, setBusy] = useState(false);

  async function upload(file: File) {
    setBusy(true);
    setError(undefined);
    setResult(undefined);
    try {
      const form = new FormData();
      form.append("file", file);
      setResult(await api<KindleImportResult>("/imports/kindle", { method: "POST", body: form }));
      onImported();
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setBusy(false);
      if (input.current) input.current.value = "";
    }
  }

  return (
    <section className="card p-4">
      <h2 className="font-semibold">Import Kindle highlights</h2>
      <p className="mt-1 text-sm text-muted">
        Connect your Kindle by USB and pick <code>documents/My Clippings.txt</code>. Highlights come in private.
      </p>
      <input
        ref={input}
        type="file"
        accept=".txt,text/plain"
        disabled={busy}
        onChange={(e) => e.target.files?.[0] && upload(e.target.files[0])}
        className="mt-3 text-sm file:mr-3 file:rounded-md file:border file:border-line file:bg-card file:px-3 file:py-1.5"
      />
      {busy && <p className="mt-2 text-sm text-muted">Importing…</p>}
      {result && (
        <p className="mt-2 text-sm">
          {result.imported} new highlight{result.imported === 1 ? "" : "s"} from {result.books} book
          {result.books === 1 ? "" : "s"}
          {result.duplicates > 0 && ` · ${result.duplicates} already imported`}
          {result.skipped_too_long > 0 && ` · ${result.skipped_too_long} skipped (over 300 words)`}
        </p>
      )}
      {error && <p className="mt-2 text-sm text-danger">{error}</p>}
    </section>
  );
}

// ---------------------------------------------------------------------------

function fileTypeOf(file: File): FileType | undefined {
  const name = file.name.toLowerCase();
  if (file.type === "application/epub+zip" || name.endsWith(".epub")) return "epub";
  if (file.type === "application/pdf" || name.endsWith(".pdf")) return "pdf";
}

export function BookRow({
  item,
  userId,
  onChange,
}: {
  item: LibraryItem;
  userId: string;
  onChange: (item: LibraryItem) => void;
}) {
  const input = useRef<HTMLInputElement>(null);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string>();

  async function upload(file: File) {
    const type = fileTypeOf(file);
    if (!type) {
      setError("Only EPUB and PDF files are supported.");
      return;
    }
    setBusy(true);
    setError(undefined);
    try {
      const path = `${userId}/${item.id}.${type}`;
      const { error } = await supabase()
        .storage.from(BOOKS_BUCKET)
        .upload(path, file, {
          upsert: true,
          contentType: type === "epub" ? "application/epub+zip" : "application/pdf",
        });
      if (error) throw error;
      onChange(await patch<LibraryItem>(`/library/${item.id}`, { file_path: path, file_type: type }));
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setBusy(false);
      if (input.current) input.current.value = "";
    }
  }

  const pct = Math.round(item.progress * 100);
  return (
    <li className="flex gap-4 py-4">
      <Cover url={item.work.cover_url} title={item.work.title} />
      <div className="min-w-0 flex-1">
        <Link href={`/read/${item.id}`} className="font-serif text-lg font-semibold hover:text-accent">
          {item.work.title}
        </Link>
        <p className="text-sm text-muted">{item.work.authors.join(", ")}</p>
        <div className="mt-2 flex items-center gap-2 text-xs text-muted">
          <div className="h-1.5 w-32 overflow-hidden rounded-full bg-line">
            <div className="h-full bg-accent" style={{ width: `${pct}%` }} />
          </div>
          {pct}% · {item.quote_count} quote{item.quote_count === 1 ? "" : "s"}
          {item.file_type && ` · ${item.file_type.toUpperCase()}`}
        </div>
        <div className="mt-3 flex flex-wrap gap-2">
          <Link href={`/read/${item.id}`} className="btn">
            {item.file_path ? "Read" : "Quotes"}
          </Link>
          <button className="btn" disabled={busy} onClick={() => input.current?.click()}>
            {busy ? "Uploading…" : item.file_path ? "Replace file" : "Upload your copy (EPUB/PDF)"}
          </button>
          <input
            ref={input}
            type="file"
            accept=".epub,.pdf,application/epub+zip,application/pdf"
            hidden
            onChange={(e) => e.target.files?.[0] && upload(e.target.files[0])}
          />
        </div>
        {error && <p className="mt-2 text-sm text-danger">{error}</p>}
      </div>
    </li>
  );
}
