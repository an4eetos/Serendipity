"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { useEffect, useState } from "react";
import { get } from "@/lib/api";
import { useSession } from "@/lib/auth";
import { supabase } from "@/lib/supabase";
import type { Profile } from "@/lib/types";

export function Header() {
  const session = useSession();
  const router = useRouter();
  const [username, setUsername] = useState<string>();

  useEffect(() => {
    if (!session) return;
    get<Profile>("/me")
      .then((p) => setUsername(p.username))
      .catch(() => {});
  }, [session]);

  async function signOut() {
    await supabase().auth.signOut();
    setUsername(undefined);
    router.push("/");
  }

  return (
    <header className="border-b border-line">
      <nav className="mx-auto flex max-w-5xl items-center gap-5 px-4 py-3 text-sm">
        <Link href="/" className="font-serif text-lg font-semibold tracking-tight">
          Serendipity
        </Link>
        <div className="flex-1" />
        {session ? (
          <>
            <Link href="/library" className="hover:text-accent">
              Library
            </Link>
            {username && (
              <Link href={`/u/${username}`} className="hover:text-accent">
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
    </header>
  );
}
