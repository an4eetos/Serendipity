"use client";

import Link from "next/link";
import { useCallback, useEffect, useState } from "react";
import { ApiError, get, post } from "@/lib/api";
import { useRequireSession } from "@/lib/auth";
import {
  reportReasonLabels,
  type Conversation,
  type Quote,
  type ReportTargetKind,
  type ReportedTarget,
  type Takedown,
  type TakedownWithTarget,
} from "@/lib/types";

const TAKEDOWN_REASON_MAX = 500;

const kindLabels: Record<ReportTargetKind, string> = { quote: "Quote", conversation: "Conversation" };

function formatDate(s: string) {
  return new Date(s).toLocaleString();
}

// AdminReports is the moderation queue: open reports grouped by target, and hidden content.
export function AdminReports() {
  const session = useRequireSession();
  const [reports, setReports] = useState<ReportedTarget[]>();
  const [takedowns, setTakedowns] = useState<TakedownWithTarget[]>();
  const [forbidden, setForbidden] = useState(false);
  const [error, setError] = useState<string>();

  const load = useCallback(() => {
    Promise.all([get<ReportedTarget[]>("/admin/reports"), get<TakedownWithTarget[]>("/admin/takedowns")])
      .then(([r, t]) => {
        setReports(r ?? []);
        setTakedowns(t ?? []);
        setError(undefined);
      })
      .catch((e: Error) => {
        if (e instanceof ApiError && e.status === 403) setForbidden(true);
        else setError(e.message);
      });
  }, []);

  useEffect(() => {
    if (session) load();
  }, [session, load]);

  if (forbidden) return <p className="text-muted">Admins only.</p>;
  if (error) return <p className="text-danger">{error}</p>;
  if (!session || !reports || !takedowns) return <p className="text-muted">Loading…</p>;

  return (
    <div className="space-y-8">
      <h1 className="font-serif text-3xl font-semibold">Reports</h1>

      <section className="space-y-3">
        <h2 className="font-semibold">Open reports</h2>
        {reports.length === 0 ? (
          <p className="text-sm text-muted">No open reports.</p>
        ) : (
          <ul className="space-y-4">
            {reports.map((t) => (
              <ReportedTargetCard key={`${t.target_kind}:${t.target_id}`} target={t} onDone={load} />
            ))}
          </ul>
        )}
      </section>

      <section className="space-y-3">
        <h2 className="font-semibold">Hidden content</h2>
        {takedowns.length === 0 ? (
          <p className="text-sm text-muted">Nothing is hidden right now.</p>
        ) : (
          <ul className="space-y-4">
            {takedowns.map((t) => (
              <TakedownCard key={t.id} takedown={t} onDone={load} />
            ))}
          </ul>
        )}
      </section>
    </div>
  );
}

// ---------------------------------------------------------------------------

function TargetHeading({ kind, ownerUsername }: { kind: ReportTargetKind; ownerUsername: string }) {
  return (
    <p className="text-sm text-muted">
      {kindLabels[kind]} by{" "}
      {ownerUsername ? (
        <Link href={`/u/${ownerUsername}`} className="text-accent hover:underline">
          @{ownerUsername}
        </Link>
      ) : (
        "an unknown reader"
      )}
    </p>
  );
}

function TargetContent({ quote, conversation }: { quote: Quote | null; conversation: Conversation | null }) {
  if (!quote && !conversation) return <p className="text-sm text-muted">The content is no longer available.</p>;
  return (
    <div className="space-y-3">
      {quote && (
        <div>
          <blockquote className="quote-text border-l-2 border-line pl-3 break-words whitespace-pre-wrap">
            {quote.text}
          </blockquote>
          <p className="mt-1 text-xs text-muted">
            {quote.work ? quote.work.title : "Unknown book"}
            {quote.location && ` · ${quote.location}`}
            {quote.passage_hidden && " · passage hidden publicly"}
          </p>
          {quote.reaction && <p className="mt-2 text-sm break-words whitespace-pre-wrap">{quote.reaction}</p>}
        </div>
      )}
      {conversation && (
        <div className="rounded-md bg-accent-soft p-3 text-sm">
          <p className="font-medium">With {conversation.persona.name}</p>
          <ul className="mt-2 max-h-64 space-y-2 overflow-y-auto">
            {conversation.messages.map((m) => (
              <li key={m.id} className="break-words whitespace-pre-wrap">
                <span className="font-medium">{m.role === "user" ? "Reader" : conversation.persona.name}:</span>{" "}
                {m.content}
              </li>
            ))}
          </ul>
        </div>
      )}
    </div>
  );
}

// ---------------------------------------------------------------------------

function ReportedTargetCard({ target, onDone }: { target: ReportedTarget; onDone: () => void }) {
  const [reason, setReason] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string>();

  async function run(action: () => Promise<unknown>) {
    setBusy(true);
    setError(undefined);
    try {
      await action();
      onDone();
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setBusy(false);
    }
  }

  const ref = { target_kind: target.target_kind, target_id: target.target_id };
  const hide = () => run(() => post<Takedown>("/admin/takedowns", { ...ref, reason: reason.trim() }));
  const dismiss = () => run(() => post<{ dismissed: number }>("/admin/reports/dismiss", ref));

  return (
    <li className="card space-y-3 p-4">
      <TargetHeading kind={target.target_kind} ownerUsername={target.owner_username} />
      <TargetContent quote={target.quote} conversation={target.conversation} />

      <div>
        <h3 className="text-sm font-medium">
          {target.reports.length} report{target.reports.length === 1 ? "" : "s"}
        </h3>
        <ul className="mt-1 divide-y divide-line text-sm">
          {target.reports.map((r) => (
            <li key={r.id} className="py-2">
              <span className="font-medium">{reportReasonLabels[r.reason] ?? r.reason}</span>
              <span className="text-muted"> · {formatDate(r.created_at)}</span>
              {r.note && <p className="mt-1 break-words whitespace-pre-wrap text-muted">{r.note}</p>}
            </li>
          ))}
        </ul>
      </div>

      <div className="flex flex-col gap-2 sm:flex-row">
        <input
          value={reason}
          onChange={(e) => setReason(e.target.value)}
          maxLength={TAKEDOWN_REASON_MAX}
          placeholder="Reason shown to the owner (required to hide)"
          aria-label="Reason for hiding"
          className="input"
        />
        <div className="flex gap-2">
          <button className="btn btn-primary" disabled={busy || !reason.trim()} onClick={hide}>
            Hide
          </button>
          <button className="btn" disabled={busy} onClick={dismiss}>
            Dismiss
          </button>
        </div>
      </div>
      {error && <p className="text-sm text-danger">{error}</p>}
    </li>
  );
}

function TakedownCard({ takedown, onDone }: { takedown: TakedownWithTarget; onDone: () => void }) {
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string>();

  async function restore() {
    setBusy(true);
    setError(undefined);
    try {
      await post<Takedown>(`/admin/takedowns/${encodeURIComponent(takedown.id)}/restore`, {});
      onDone();
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setBusy(false);
    }
  }

  return (
    <li className="card space-y-3 p-4">
      <TargetHeading kind={takedown.target_kind} ownerUsername={takedown.owner_username} />
      <p className="text-sm">
        <span className="font-medium">Removed:</span> <span className="break-words">{takedown.reason}</span>
        <span className="text-muted"> · {formatDate(takedown.created_at)}</span>
      </p>
      <TargetContent quote={takedown.quote} conversation={takedown.conversation} />
      <div>
        {/* Without content (the owner deleted it), restoring only closes the takedown. */}
        <button className="btn" disabled={busy} onClick={restore}>
          {takedown.quote || takedown.conversation ? "Restore" : "Close"}
        </button>
      </div>
      {error && <p className="text-sm text-danger">{error}</p>}
    </li>
  );
}
