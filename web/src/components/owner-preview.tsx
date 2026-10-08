"use client";

import { useEffect, useState } from "react";
import { get } from "@/lib/api";
import { useSession } from "@/lib/auth";
import type { PublicProfile } from "@/lib/types";
import { ProfileView } from "./profile-view";

// Shown when the public profile is not found: if the visitor owns it (it's
// private), render a preview with their own token.
export function OwnerPreview({ username }: { username: string }) {
  const session = useSession();
  const [data, setData] = useState<PublicProfile | null>();

  useEffect(() => {
    if (!session) return;
    get<PublicProfile>(`/profiles/${encodeURIComponent(username)}`)
      .then(setData)
      .catch(() => setData(null));
  }, [session, username]);

  if (session === undefined || (session && data === undefined)) return <p className="text-muted">Loading…</p>;
  if (!session || !data) {
    return (
      <div className="py-12 text-center">
        <h1 className="font-serif text-2xl font-semibold">@{username}</h1>
        <p className="mt-2 text-muted">This profile is private or doesn&apos;t exist.</p>
      </div>
    );
  }
  return <ProfileView data={data} preview={!data.profile.is_public} />;
}
