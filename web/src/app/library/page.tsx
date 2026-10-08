"use client";

import { useCallback, useEffect, useState } from "react";
import { AddBook, BookRow, KindleImport, ProfileCard } from "@/components/library";
import { get } from "@/lib/api";
import { useRequireSession } from "@/lib/auth";
import type { LibraryItem, Profile } from "@/lib/types";

export default function LibraryPage() {
  const session = useRequireSession();
  const [profile, setProfile] = useState<Profile>();
  const [items, setItems] = useState<LibraryItem[]>();
  const [error, setError] = useState<string>();

  const load = useCallback(() => {
    Promise.all([get<Profile>("/me"), get<LibraryItem[]>("/library")])
      .then(([p, l]) => {
        setProfile(p);
        setItems(l);
      })
      .catch((e: Error) => setError(e.message));
  }, []);

  useEffect(() => {
    if (session) load();
  }, [session, load]);

  function upsert(item: LibraryItem) {
    setItems((prev) => [item, ...(prev ?? []).filter((i) => i.id !== item.id)]);
  }

  if (error) return <p className="text-danger">{error}</p>;
  if (!session || !profile || !items) return <p className="text-muted">Loading…</p>;

  return (
    <div className="space-y-6">
      <h1 className="font-serif text-3xl font-semibold">Your library</h1>
      <ProfileCard profile={profile} onChange={setProfile} />
      <div className="grid gap-6 md:grid-cols-2">
        <AddBook onAdded={upsert} />
        <KindleImport onImported={load} />
      </div>
      {items.length === 0 ? (
        <p className="text-muted">No books yet. Add one above, or import your Kindle highlights.</p>
      ) : (
        <ul className="divide-y divide-line">
          {items.map((item) => (
            <BookRow
              key={item.id}
              item={item}
              userId={session.user.id}
              onChange={(updated) => setItems((prev) => prev?.map((i) => (i.id === updated.id ? updated : i)))}
            />
          ))}
        </ul>
      )}
    </div>
  );
}
