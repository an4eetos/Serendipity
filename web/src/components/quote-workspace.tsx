"use client";

import { useEffect, useRef, useState } from "react";
import { del, get, patch, stream } from "@/lib/api";
import type { Conversation, Message, Persona, Quote, QuoteWithConversations } from "@/lib/types";

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
      {quote.removed_at && (
        <div className="rounded-md border border-danger px-3 py-2 text-sm text-danger">
          <p className="font-medium">Removed: {quote.removed_reason}</p>
          <p className="mt-1 text-xs">
            A moderator took this quote down. It&apos;s hidden from public pages and can&apos;t be published.
          </p>
        </div>
      )}
      <blockquote className="border-l-2 border-accent pl-4">
        <p className="quote-text whitespace-pre-wrap">{quote.text}</p>
        <div className="mt-2">
          <Citation quote={quote} />
        </div>
      </blockquote>

      <ReactionEditor quote={quote} onSaved={updateQuote} onDeleted={onDeleted} />

      <Discussions
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

function ReactionEditor({
  quote,
  onSaved,
  onDeleted,
}: {
  quote: Quote;
  onSaved: (q: Quote) => void;
  onDeleted?: () => void;
}) {
  const [reaction, setReaction] = useState(quote.reaction);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string>();
  const dirty = reaction !== quote.reaction;

  async function save(body: { reaction?: string; visibility?: "public" | "private" }) {
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
    if (!confirm("Delete this quote, your reaction and its discussions?")) return;
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
  const removed = quote.removed_at != null;
  return (
    <section className="space-y-2">
      <label className="font-medium" htmlFor={`reaction-${quote.id}`}>
        Your reaction
      </label>
      <textarea
        id={`reaction-${quote.id}`}
        value={reaction}
        onChange={(e) => setReaction(e.target.value)}
        rows={4}
        placeholder="What does this passage do to you? Agree, disagree, connect it to something — this is what you share."
        className="input resize-y text-[0.95rem]"
      />
      <div className="flex flex-wrap items-center gap-2">
        <button className="btn" disabled={!dirty || busy} onClick={() => save({ reaction })}>
          Save reaction
        </button>
        <button
          className={`btn ${isPublic ? "" : "btn-primary"}`}
          disabled={busy || (removed && !isPublic)}
          onClick={() =>
            save({ reaction: dirty ? reaction : undefined, visibility: isPublic ? "private" : "public" })
          }
          title={
            isPublic
              ? undefined
              : removed
                ? "A moderator removed this quote, so it can't be published"
                : "Shows the quote, your reaction and its discussions on your profile"
          }
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
      {isPublic && quote.passage_hidden && (
        <p className="rounded-md bg-accent-soft px-3 py-2 text-sm text-accent">
          On public pages this passage is hidden because this book&apos;s public quota is full. Your reaction and the
          citation still show.
        </p>
      )}
      {error && <p className="text-sm text-danger">{error}</p>}
    </section>
  );
}

// ---------------------------------------------------------------------------

type SetConversations = (fn: (prev: Conversation[]) => Conversation[]) => void;

function Discussions({
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
  const [open, setOpen] = useState(false);
  const [personaId, setPersonaId] = useState<string>();
  const [pending, setPending] = useState<{ persona: Persona; text: string }>();
  const [error, setError] = useState<string>();

  const persona = personas.find((p) => p.id === (personaId ?? personas[0]?.id));
  const hasReaction = quote.reaction.trim() !== "";

  async function start() {
    if (!persona) return;
    setError(undefined);
    setPending({ persona, text: "" });
    let convId = "";
    try {
      await stream("/conversations", { quote_id: quote.id, persona_id: persona.id }, (e) => {
        if (e.type === "start") convId = e.conversation_id;
        if (e.type === "delta") setPending((p) => p && { ...p, text: p.text + e.text });
        if (e.type === "done") {
          setConversations((prev) => [
            ...prev,
            {
              id: convId,
              quote_id: quote.id,
              persona,
              messages: [e.message],
              created_at: e.message.created_at,
              removed_at: null,
              removed_reason: "",
            },
          ]);
          setOpen(false);
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
      {conversations.length > 0 && <h3 className="text-sm font-medium">Discussions</h3>}

      {conversations.map((c) => (
        <Thread key={c.id} conversation={c} setConversations={setConversations} />
      ))}

      {pending && (
        <div className="card space-y-3 p-3">
          <p className="font-medium">{pending.persona.name}</p>
          <MessageBubble
            message={{ role: "assistant", content: pending.text || "…" }}
            personaName={pending.persona.name}
          />
        </div>
      )}

      {!hasReaction ? (
        <p className="text-sm text-muted">
          Once you&apos;ve saved your reaction, you can discuss it with a character if you want.
        </p>
      ) : !open ? (
        <button className="btn" disabled={!!pending} onClick={() => setOpen(true)}>
          {conversations.length ? "Discuss with someone else" : "Discuss your reaction with a character"}
        </button>
      ) : (
        <div className="card space-y-3 p-3">
          <p className="text-sm text-muted">Who do you want to talk this over with?</p>
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
          <div className="flex gap-2">
            <button className="btn btn-primary" disabled={!persona || !!pending} onClick={start}>
              Discuss with {persona?.name ?? "…"}
            </button>
            <button className="btn" disabled={!!pending} onClick={() => setOpen(false)}>
              Cancel
            </button>
          </div>
        </div>
      )}
      {error && <p className="text-sm text-danger">{error}</p>}
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
      <p className="font-medium">{name}</p>
      {conversation.removed_at && (
        <p className="rounded-md border border-danger px-3 py-2 text-sm text-danger">
          Removed: {conversation.removed_reason}
        </p>
      )}
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
