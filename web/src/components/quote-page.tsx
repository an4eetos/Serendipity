"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { useRequireSession } from "@/lib/auth";
import { QuoteWorkspace } from "./quote-workspace";

export function QuotePage({ id }: { id: string }) {
  const session = useRequireSession();
  const router = useRouter();
  if (!session) return <p className="text-muted">Loading…</p>;
  return (
    <div className="mx-auto max-w-2xl space-y-4">
      <button onClick={() => router.back()} className="text-sm text-muted hover:text-ink">
        ← Back
      </button>
      <QuoteWorkspace quoteId={id} onDeleted={() => router.push("/library")} />
      <p className="text-xs text-muted">
        Public quotes appear on your <Link href="/library" className="underline">profile</Link> with your note and these
        conversations.
      </p>
    </div>
  );
}
