"use client";

import { useId, useState } from "react";
import { post } from "@/lib/api";
import { useSession } from "@/lib/auth";
import { reportReasonLabels, type Report, type ReportReason, type ReportTargetKind } from "@/lib/types";

const MAX_NOTE = 1000;
const reasons = Object.keys(reportReasonLabels) as ReportReason[];

type Props = {
  targetKind: ReportTargetKind;
  targetId: string;
  // The content owner's user id: owners can't report their own content.
  ownerId: string;
};

// ReportButton lets a signed-in reader flag a public quote or discussion for
// moderators. It renders nothing for signed-out viewers and for the owner.
export function ReportButton({ targetKind, targetId, ownerId }: Props) {
  const session = useSession();
  const id = useId();
  const [open, setOpen] = useState(false);
  const [reason, setReason] = useState<ReportReason | "">("");
  const [note, setNote] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string>();
  const [sent, setSent] = useState(false);

  if (!session || session.user.id === ownerId) return null;

  if (sent) return <p className="text-xs text-muted">Thanks for the report. A moderator will take a look.</p>;

  async function submit(e: React.FormEvent) {
    e.preventDefault();
    if (!reason) return;
    setBusy(true);
    setError(undefined);
    try {
      await post<Report>("/reports", { target_kind: targetKind, target_id: targetId, reason, note: note.trim() });
      setSent(true);
    } catch (err) {
      setError((err as Error).message);
    } finally {
      setBusy(false);
    }
  }

  if (!open) {
    return (
      <button type="button" className="text-xs text-muted hover:text-danger" onClick={() => setOpen(true)}>
        Report
      </button>
    );
  }

  return (
    <form onSubmit={submit} className="space-y-2 rounded-md border border-line p-3">
      <p className="text-sm font-medium">
        Report this {targetKind === "conversation" ? "discussion" : "quote"}
      </p>
      <label htmlFor={`${id}-reason`} className="block text-xs text-muted">
        Reason
      </label>
      <select
        id={`${id}-reason`}
        value={reason}
        onChange={(e) => setReason(e.target.value as ReportReason)}
        className="input"
        required
      >
        <option value="" disabled>
          Choose a reason
        </option>
        {reasons.map((r) => (
          <option key={r} value={r}>
            {reportReasonLabels[r]}
          </option>
        ))}
      </select>
      <label htmlFor={`${id}-note`} className="block text-xs text-muted">
        Note for moderators (optional)
      </label>
      <textarea
        id={`${id}-note`}
        value={note}
        onChange={(e) => setNote(e.target.value)}
        maxLength={MAX_NOTE}
        rows={3}
        className="input resize-y"
      />
      <div className="flex flex-wrap items-center gap-2">
        <button className="btn btn-primary" disabled={busy || !reason}>
          Send report
        </button>
        <button
          type="button"
          className="btn"
          disabled={busy}
          onClick={() => {
            setOpen(false);
            setError(undefined);
          }}
        >
          Cancel
        </button>
      </div>
      {error && <p className="text-sm text-danger">{error}</p>}
    </form>
  );
}
