"use client";

import { useEffect, useRef, useState } from "react";
import { del, get, patch, stream } from "@/lib/api";
import type { Conversation, Message, Mode, Persona, Quote, QuoteWithConversations } from "@/lib/types";

let personasCache: Promise<Persona[]> | undefined;
function loadPersonas() {
  personasCache ??= get<Persona[]>("/personas").catch((e) => {
    personasCache = undefined;
    throw e;
  });
  return personasCache;
}

export function Citation({ quote }: { quote: Quote }) {
  return (
    <p className="text-sm text-muted">
      — <span className="italic">{quote.work?.title}</span>
      {quote.work?.authors.length ? `, ${quote.work.authors.join(", ")}` : ""}
      {quote.location && ` · ${quote.location}`}
    </p>
  );
}

export function MessageBubble({ message, personaName }: { message: Pick<Message, "role" | "content">; personaName: string }) {
  const mine = message.role === "user";
  return (
    <div className={mine ? "ml-8" : "mr-4"}>
      <p className="mb-0.5 text-xs font-medium text-muted">{mine ? "You" : personaName}</p>
      <div
        className={`whitespace-pre-wrap rounded-lg px-3 py-2 text-[0.95rem] leading-relaxed ${
          mine ? "bg-accent-soft" : "border border-line bg-card"
        }`}
      >
        {message.content}
      </div>
    </div>
  );
}

function ModeBadge({ mode }: { mode: Mode }) {
  return (
    <span className="rounded-full border border-line px-2 py-0.5 text-xs text-muted">
      {mode === "argue" ? "argument" : "reaction"}
    </span>
  );
}

type Props = {
  quoteId: string;
  onChange?: (q: Quote) => void;
  onDeleted?: () => void;
};

export function QuoteWorkspace({ quoteId, onChange, onDeleted }: Props) {
  const [quote, setQuote] = useState<QuoteWithConversations>();
  const [personas, setPersonas] = useState<Persona[]>([]);
  const [error, setError] = useState<string>();

  useEffect(() => {
    // Callers key this component by quote id, so quoteId never changes here.
    let cancelled = false;
    Promise.all([get<QuoteWithConversations>(`/quotes/${quoteId}`), loadPersonas()])
      .then(([q, p]) => {
        if (cancelled) return;
        setQuote(q);
        setPersonas(p);
      })
      .catch((e: Error) => !cancelled && setError(e.message));
    return () => {
      cancelled = true;
    };
  }, [quoteId]);

  if (error) return <p className="text-sm text-danger">{error}</p>;
  if (!quote) return <p className="text-sm text-muted">Loading…</p>;

  function updateQuote(q: Quote) {
    setQuote((prev) => (prev ? { ...prev, ...q } : prev));
    onChange?.(q);
  }

  return (
    <div className="space-y-6">
      <blockquote className="border-l-2 border-accent pl-4">
        <p className="quote-text whitespace-pre-wrap">{quote.text}</p>
        <div className="mt-2">
          <Citation quote={quote} />
        </div>
      </blockquote>

      <NoteEditor quote={quote} onSaved={updateQuote} onDeleted={onDeleted} />

      <Conversations
        quote={quote}
        personas={personas}
        conversations={quote.conversations}
        setConversations={(fn) =>
          setQuote((prev) => (prev ? { ...prev, conversations: fn(prev.conversations) } : prev))
        }
      />
    </div>
  );
}

// ---------------------------------------------------------------------------

function NoteEditor({
  quote,
  onSaved,
  onDeleted,
}: {
  quote: Quote;
  onSaved: (q: Quote) => void;
  onDeleted?: () => void;
}) {
  const [note, setNote] = useState(quote.note);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string>();
  const dirty = note !== quote.note;

  async function save(body: { note?: string; visibility?: "public" | "private" }) {
    setBusy(true);
    setError(undefined);
    try {
      onSaved(await patch<Quote>(`/quotes/${quote.id}`, body));
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setBusy(false);
    }
  }

  async function remove() {
    if (!confirm("Delete this quote and its conversations?")) return;
    setBusy(true);
    try {
      await del(`/quotes/${quote.id}`);
      onDeleted?.();
    } catch (e) {
      setError((e as Error).message);
      setBusy(false);
    }
  }

  const isPublic = quote.visibility === "public";
  return (
    <section className="space-y-2">
      <label className="text-sm font-medium" htmlFor={`note-${quote.id}`}>
        Your note
      </label>
      <textarea
        id={`note-${quote.id}`}
        value={note}
        onChange={(e) => setNote(e.target.value)}
        rows={3}
        placeholder="What does this make you think? Your perspective is what gets published."
        className="input resize-y"
      />
      <div className="flex flex-wrap items-center gap-2">
        <button className="btn" disabled={!dirty || busy} onClick={() => save({ note })}>
          Save note
        </button>
        <button
          className={`btn ${isPublic ? "" : "btn-primary"}`}
          disabled={busy}
          onClick={() =>
            save({ note: dirty ? note : undefined, visibility: isPublic ? "private" : "public" })
          }
          title={isPublic ? undefined : "Shows the quote, your note and its conversations on your profile"}
        >
          {isPublic ? "Make private" : "Publish to profile"}
        </button>
        <span className="text-xs text-muted">{isPublic ? "Public" : "Private"}</span>
        <div className="flex-1" />
        {onDeleted && (
          <button className="text-xs text-muted hover:text-danger" disabled={busy} onClick={remove}>
            Delete quote
          </button>
        )}
      </div>
      {error && <p className="text-sm text-danger">{error}</p>}
    </section>
  );
}

// ---------------------------------------------------------------------------

type SetConversations = (fn: (prev: Conversation[]) => Conversation[]) => void;

function Conversations({
  quote,
  personas,
  conversations,
  setConversations,
}: {
  quote: Quote;
  personas: Persona[];
  conversations: Conversation[];
  setConversations: SetConversations;
}) {
  const [personaId, setPersonaId] = useState<string>();
  const [mode, setMode] = useState<Mode>("react");
  const [pending, setPending] = useState<{ persona: Persona; mode: Mode; text: string }>();
  const [error, setError] = useState<string>();

  const persona = personas.find((p) => p.id === (personaId ?? personas[0]?.id));

  async function start() {
    if (!persona) return;
    setError(undefined);
    setPending({ persona, mode, text: "" });
    let convId = "";
    try {
      await stream("/conversations", { quote_id: quote.id, persona_id: persona.id, mode }, (e) => {
        if (e.type === "start") convId = e.conversation_id;
        if (e.type === "delta") setPending((p) => p && { ...p, text: p.text + e.text });
        if (e.type === "done") {
          setConversations((prev) => [
            ...prev,
            { id: convId, quote_id: quote.id, mode, persona, messages: [e.message], created_at: e.message.created_at },
          ]);
        }
        if (e.type === "error") setError(e.error);
      });
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setPending(undefined);
    }
  }

  return (
    <section className="space-y-4">
      <h3 className="text-sm font-medium">Conversations</h3>

      {conversations.map((c) => (
        <Thread key={c.id} conversation={c} setConversations={setConversations} />
      ))}

      {pending && (
        <div className="card space-y-3 p-3">
          <div className="flex items-center gap-2">
            <span className="font-medium">{pending.persona.name}</span>
            <ModeBadge mode={pending.mode} />
          </div>
          <MessageBubble
            message={{ role: "assistant", content: pending.text || "…" }}
            personaName={pending.persona.name}
          />
        </div>
      )}

      <div className="card space-y-3 p-3">
        <div className="flex flex-wrap gap-1.5">
          {personas.map((p) => (
            <button
              key={p.id}
              onClick={() => setPersonaId(p.id)}
              title={`${p.era} — ${p.short_bio}`}
              className={`rounded-full border px-3 py-1 text-sm transition ${
                p.id === persona?.id ? "border-accent bg-accent-soft text-accent" : "border-line hover:border-muted"
              }`}
            >
              {p.name}
            </button>
          ))}
        </div>
        {persona && <p className="text-xs text-muted">{persona.short_bio}</p>}
        <div className="flex flex-wrap items-center gap-2">
          <div className="inline-flex overflow-hidden rounded-md border border-line text-sm">
            {(["react", "argue"] as Mode[]).map((m) => (
              <button
                key={m}
                onClick={() => setMode(m)}
                className={`px-3 py-1.5 ${mode === m ? "bg-accent-soft text-accent" : ""}`}
              >
                {m === "react" ? "React" : "Argue"}
              </button>
            ))}
          </div>
          <button className="btn btn-primary" disabled={!persona || !!pending} onClick={start}>
            {mode === "react" ? `Ask ${persona?.name ?? "…"} to react` : `Argue with ${persona?.name ?? "…"}`}
          </button>
        </div>
        {error && <p className="text-sm text-danger">{error}</p>}
      </div>
    </section>
  );
}

function Thread({ conversation, setConversations }: { conversation: Conversation; setConversations: SetConversations }) {
  const [reply, setReply] = useState("");
  const [pending, setPending] = useState<string>();
  const [error, setError] = useState<string>();
  const bottom = useRef<HTMLDivElement>(null);
  const name = conversation.persona.name;

  function update(fn: (c: Conversation) => Conversation) {
    setConversations((prev) => prev.map((c) => (c.id === conversation.id ? fn(c) : c)));
  }

  async function send(e: React.FormEvent) {
    e.preventDefault();
    const content = reply.trim();
    if (!content) return;
    setError(undefined);
    setReply("");
    const temp: Message = { id: -Date.now(), role: "user", content, created_at: new Date().toISOString() };
    update((c) => ({ ...c, messages: [...c.messages, temp] }));
    setPending("");
    let ok = false;
    try {
      await stream(`/conversations/${conversation.id}/messages`, { content }, (ev) => {
        if (ev.type === "delta") setPending((p) => (p ?? "") + ev.text);
        if (ev.type === "done") {
          ok = true;
          update((c) => ({ ...c, messages: [...c.messages, ev.message] }));
        }
        if (ev.type === "error") setError(ev.error);
      });
    } catch (err) {
      setError((err as Error).message);
    } finally {
      setPending(undefined);
      if (!ok) {
        // The server dropped the unanswered message; put it back in the box.
        update((c) => ({ ...c, messages: c.messages.filter((m) => m.id !== temp.id) }));
        setReply(content);
      }
    }
  }

  useEffect(() => {
    bottom.current?.scrollIntoView({ block: "nearest" });
  }, [conversation.messages.length, pending]);

  return (
    <div className="card space-y-3 p-3">
      <div className="flex items-center gap-2">
        <span className="font-medium">{name}</span>
        <ModeBadge mode={conversation.mode} />
      </div>
      {conversation.messages.map((m) => (
        <MessageBubble key={m.id} message={m} personaName={name} />
      ))}
      {pending !== undefined && <MessageBubble message={{ role: "assistant", content: pending || "…" }} personaName={name} />}
      <form onSubmit={send} className="flex gap-2">
        <input
          value={reply}
          onChange={(e) => setReply(e.target.value)}
          placeholder={`Answer ${name}…`}
          disabled={pending !== undefined}
          className="input"
        />
        <button className="btn" disabled={pending !== undefined || !reply.trim()}>
          Send
        </button>
      </form>
      {error && <p className="text-sm text-danger">{error}</p>}
      <div ref={bottom} />
    </div>
  );
}
