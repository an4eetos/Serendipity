"use client";

import { useRouter } from "next/navigation";
import { useEffect, useState } from "react";
import { useSession } from "@/lib/auth";
import { supabase } from "@/lib/supabase";

export default function LoginPage() {
  const session = useSession();
  const router = useRouter();
  const [email, setEmail] = useState("");
  const [state, setState] = useState<"idle" | "sending" | "sent">("idle");
  const [error, setError] = useState<string>();

  useEffect(() => {
    if (session) router.replace("/library");
  }, [session, router]);

  async function submit(e: React.FormEvent) {
    e.preventDefault();
    setState("sending");
    setError(undefined);
    const { error } = await supabase().auth.signInWithOtp({
      email,
      options: { emailRedirectTo: `${window.location.origin}/library` },
    });
    if (error) {
      setError(error.message);
      setState("idle");
    } else {
      setState("sent");
    }
  }

  return (
    <div className="mx-auto max-w-sm py-12">
      <h1 className="font-serif text-3xl font-semibold">Sign in</h1>
      {state === "sent" ? (
        <p className="mt-4 text-muted">
          Check <strong className="text-ink">{email}</strong> for a sign-in link.
        </p>
      ) : (
        <form onSubmit={submit} className="mt-6 space-y-3">
          <input
            type="email"
            required
            autoFocus
            placeholder="you@example.com"
            value={email}
            onChange={(e) => setEmail(e.target.value)}
            className="input"
          />
          <button type="submit" disabled={state === "sending"} className="btn btn-primary w-full py-2">
            {state === "sending" ? "Sending…" : "Email me a sign-in link"}
          </button>
          {error && <p className="text-sm text-danger">{error}</p>}
        </form>
      )}
    </div>
  );
}
