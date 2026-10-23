"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { useState } from "react";
import { post } from "@/lib/api";
import type { Me } from "@/lib/types";

// TermsPrompt asks the reader to accept the current terms. It blocks the page,
// except on /terms, which the reader must be able to read in full first, and on
// admin pages, which the terms don't gate. There it is an inline banner.
// It reads the pathname, so render it inside a <Suspense> boundary.
export function TermsPrompt({ me, onAccepted }: { me: Me; onAccepted: (me: Me) => void }) {
  const pathname = usePathname();
  const onTermsPage = pathname === "/terms";
  const inline = onTermsPage || pathname === "/admin" || pathname.startsWith("/admin/");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string>();

  async function accept() {
    setBusy(true);
    setError(undefined);
    try {
      onAccepted(await post<Me>("/me/terms", { version: me.current_terms_version }));
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setBusy(false);
    }
  }

  const request = onTermsPage ? (
    <>Read the terms below, then accept them</>
  ) : (
    <>
      Please read and accept the{" "}
      <Link href="/terms" target="_blank" rel="noopener noreferrer" className="text-accent underline">
        terms of service
      </Link>
    </>
  );
  const acceptButton = (
    <button className="btn btn-primary" disabled={busy} onClick={accept}>
      {busy ? "Saving…" : "I accept"}
    </button>
  );

  if (inline) {
    return (
      <div role="region" aria-label="Terms of service" className="border-t border-line bg-accent-soft">
        <div className="mx-auto flex max-w-5xl flex-wrap items-center gap-3 px-4 py-3 text-sm">
          <p className="min-w-0 flex-1">
            {request} (version {me.current_terms_version}) to keep using Serendipity.
          </p>
          {acceptButton}
          {error && <p className="w-full text-danger">{error}</p>}
        </div>
      </div>
    );
  }

  return (
    <div
      role="dialog"
      aria-modal="true"
      aria-labelledby="terms-prompt-title"
      className="fixed inset-0 z-50 flex items-end justify-center bg-black/50 p-4 sm:items-center"
    >
      <div className="card w-full max-w-md p-5 shadow-lg">
        <h2 id="terms-prompt-title" className="font-serif text-xl font-semibold">
          {me.terms_version ? "Our terms have changed" : "Before you continue"}
        </h2>
        <p className="mt-2 text-sm text-muted">
          {request} (version {me.current_terms_version}) to keep using Serendipity.
        </p>
        <div className="mt-4 flex justify-end">{acceptButton}</div>
        {error && <p className="mt-2 text-sm text-danger">{error}</p>}
      </div>
    </div>
  );
}
