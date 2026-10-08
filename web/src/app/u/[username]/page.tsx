import { Suspense } from "react";
import { OwnerPreview } from "@/components/owner-preview";
import { ProfileView } from "@/components/profile-view";
import type { PublicProfile } from "@/lib/types";

const API_URL = process.env.API_URL ?? process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8787";

async function Profile({ username }: { username: string }) {
  const res = await fetch(`${API_URL}/profiles/${encodeURIComponent(username)}`);
  if (res.status === 404) return <OwnerPreview username={username} />;
  if (!res.ok) return <p className="text-danger">Could not load this profile right now.</p>;
  const data = (await res.json()) as PublicProfile;
  return <ProfileView data={data} />;
}

export default function ProfilePage({ params }: PageProps<"/u/[username]">) {
  return (
    <Suspense fallback={<p className="text-muted">Loading…</p>}>
      {params.then(({ username }) => (
        <Profile username={decodeURIComponent(username)} />
      ))}
    </Suspense>
  );
}
