"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { Suspense, useEffect, useState } from "react";
import { get } from "@/lib/api";
import { useSession } from "@/lib/auth";
import { supabase } from "@/lib/supabase";
import type { Me } from "@/lib/types";
import { TermsPrompt } from "./terms-prompt";

export function Header() {
  const session = useSession();
  const router = useRouter();
  const [me, setMe] = useState<Me>();

  useEffect(() => {
    if (!session) return;
    get<Me>("/me")
      .then(setMe)
      .catch(() => {});
  }, [session]);

  async function signOut() {
    await supabase().auth.signOut();
    setMe(undefined);
    router.push("/");
  }

  return (
    <header className="border-b border-line">
      <nav className="mx-auto flex max-w-5xl flex-wrap items-center gap-x-4 gap-y-2 px-4 sm:gap-x-5 py-3 text-sm">
        <Link href="/" className="font-serif text-lg font-semibold tracking-tight">
          Serendipity
        </Link>
        <div className="flex-1" />
        {session ? (
          <>
            <Link href="/library" className="hover:text-accent">
              Library
            </Link>
            <Link href="/characters" className="hover:text-accent">
              Characters
            </Link>
            {me?.username && (
              <Link href={`/u/${me.username}`} className="hover:text-accent">
                Profile
              </Link>
            )}
            <button onClick={signOut} className="text-muted hover:text-ink">
              Sign out
            </button>
          </>
        ) : session === null ? (
          <Link href="/login" className="btn">
            Sign in
          </Link>
        ) : null}
      </nav>
      {session && me && !me.terms_accepted && (
        // TermsPrompt reads the pathname: its own boundary keeps the layout
        // from suspending on dynamic routes under cacheComponents.
        <Suspense fallback={null}>
          <TermsPrompt me={me} onAccepted={setMe} />
        </Suspense>
      )}
    </header>
  );
}
