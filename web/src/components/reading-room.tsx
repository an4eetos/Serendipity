"use client";

import dynamic from "next/dynamic";
import Link from "next/link";
import { useCallback, useEffect, useRef, useState } from "react";
import { get, patch, post } from "@/lib/api";
import { useRequireSession } from "@/lib/auth";
import type { LibraryItem, Quote } from "@/lib/types";
import { QuoteWorkspace } from "./quote-workspace";
import type { Selection } from "./readers/types";

const EpubReader = dynamic(() => import("./readers/epub-reader"), { ssr: false });
const PdfReader = dynamic(() => import("./readers/pdf-reader"), { ssr: false });

const MAX_WORDS = 300;

export function ReadingRoom({ itemId }: { itemId: string }) {
  const session = useRequireSession();
  const [item, setItem] = useState<LibraryItem>();
  const [quotes, setQuotes] = useState<Quote[]>([]);
  const [file, setFile] = useState<ArrayBuffer>();
  const [fileError, setFileError] = useState<string>();
  const [error, setError] = useState<string>();
  const [selection, setSelection] = useState<Selection | null>(null);
  const [activeQuote, setActiveQuote] = useState<string>();
  const [jumpTo, setJumpTo] = useState<string>();
  const [saving, setSaving] = useState(false);

  useEffect(() => {
    if (!session) return;
    let loaded = false;
    get<LibraryItem>(`/library/${itemId}`)
      .then(async (it) => {
        setItem(it);
        loaded = true;
        setQuotes(await get<Quote[]>(`/quotes?work_id=${it.work.id}`));
        if (!it.file_path) return;
        const { url } = await get<{ url: string }>(`/library/${itemId}/file-url`);
        const res = await fetch(url);
        if (!res.ok) throw new Error("Could not download your file");
        setFile(await res.arrayBuffer());
      })
      .catch((e: Error) => (loaded ? setFileError(e.message) : setError(e.message)));
  }, [session, itemId]);

  // Save reading progress, at most every few seconds.
  const progressTimer = useRef<ReturnType<typeof setTimeout>>(undefined);
  const onProgress = useCallback(
    (progress: number, location: string) => {
      clearTimeout(progressTimer.current);
      progressTimer.current = setTimeout(() => {
        patch(`/library/${itemId}`, { progress: Math.min(1, Math.max(0, progress)), last_location: location }).catch(
          () => {},
        );
      }, 2000);
    },
    [itemId],
  );

  async function saveSelection() {
    if (!selection || !item) return;
    setSaving(true);
    try {
      const q = await post<Quote>("/quotes", { library_item_id: item.id, ...selection });
      setQuotes((prev) => [...prev.filter((x) => x.id !== q.id), q].sort(byPosition));
      setActiveQuote(q.id);
      setSelection(null);
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setSaving(false);
    }
  }

  if (error) return <p className="text-danger">{error}</p>;
  if (!item) return <p className="text-muted">Loading…</p>;

  const words = selection ? selection.text.split(/\s+/).filter(Boolean).length : 0;
  const Reader = item.file_type === "pdf" ? PdfReader : EpubReader;

  return (
    <div className="space-y-4">
      <div>
        <Link href="/library" className="text-sm text-muted hover:text-ink">
          ← Library
        </Link>
        <h1 className="font-serif text-2xl font-semibold">{item.work.title}</h1>
        <p className="text-sm text-muted">{item.work.authors.join(", ")}</p>
      </div>

      <div className="grid gap-6 lg:grid-cols-[minmax(0,1fr)_420px]">
        <div className="card h-[75vh] p-3">
          {!item.file_path ? (
            <div className="flex h-full flex-col items-center justify-center gap-2 text-center text-sm text-muted">
              <p>No copy uploaded for this book.</p>
              <p>
                Upload your EPUB or PDF from the <Link href="/library" className="text-accent underline">library</Link>{" "}
                to read and quote here. Your file stays private to you.
              </p>
            </div>
          ) : fileError ? (
            <p className="text-sm text-danger">{fileError}</p>
          ) : !file ? (
            <p className="text-sm text-muted">Opening your copy…</p>
          ) : (
            <Reader
              data={file}
              initialLocation={item.last_location}
              jumpTo={jumpTo}
              onSelect={setSelection}
              onProgress={onProgress}
            />
          )}
        </div>

        <aside className="space-y-4">
          {selection && (
            <div className="card space-y-2 border-accent p-3">
              <p className="line-clamp-4 font-serif text-sm">&ldquo;{selection.text}&rdquo;</p>
              <p className="text-xs text-muted">
                {selection.location || "No location"} · {words} words
              </p>
              {words > MAX_WORDS ? (
                <p className="text-sm text-danger">Quotes are limited to {MAX_WORDS} words — select a shorter passage.</p>
              ) : (
                <div className="flex gap-2">
                  <button className="btn btn-primary" disabled={saving} onClick={saveSelection}>
                    Quote this
                  </button>
                  <button className="btn" onClick={() => setSelection(null)}>
                    Cancel
                  </button>
                </div>
              )}
            </div>
          )}

          {activeQuote ? (
            <div className="card p-4">
              <div className="mb-3 flex justify-between text-sm">
                <button className="text-muted hover:text-ink" onClick={() => setActiveQuote(undefined)}>
                  ← All quotes
                </button>
                <Link href={`/quotes/${activeQuote}`} className="text-muted hover:text-ink">
                  Open full page
                </Link>
              </div>
              <QuoteWorkspace
                key={activeQuote}
                quoteId={activeQuote}
                onChange={(q) => setQuotes((prev) => prev.map((x) => (x.id === q.id ? q : x)))}
                onDeleted={() => {
                  setQuotes((prev) => prev.filter((x) => x.id !== activeQuote));
                  setActiveQuote(undefined);
                }}
              />
            </div>
          ) : (
            <QuoteList
              quotes={quotes}
              onOpen={(q) => {
                setActiveQuote(q.id);
                if (q.anchor) setJumpTo(q.anchor);
              }}
            />
          )}
        </aside>
      </div>
    </div>
  );
}

function byPosition(a: Quote, b: Quote) {
  return (a.position ?? 2) - (b.position ?? 2) || a.created_at.localeCompare(b.created_at);
}

function QuoteList({ quotes, onOpen }: { quotes: Quote[]; onOpen: (q: Quote) => void }) {
  if (quotes.length === 0) {
    return <p className="text-sm text-muted">No quotes from this book yet. Select a passage to start.</p>;
  }
  return (
    <ul className="space-y-3">
      {quotes.map((q) => (
        <li key={q.id}>
          <button onClick={() => onOpen(q)} className="card block w-full p-3 text-left hover:border-muted">
            <p className="line-clamp-3 font-serif text-sm">&ldquo;{q.text}&rdquo;</p>
            <div className="mt-1 flex items-center justify-between gap-2">
              <span className="text-xs text-muted">{q.location || "No location"}</span>
              <span className={`text-xs ${q.visibility === "public" ? "text-accent" : "text-muted"}`}>
                {q.visibility}
              </span>
            </div>
          </button>
        </li>
      ))}
    </ul>
  );
}
