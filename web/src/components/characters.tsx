"use client";

import Link from "next/link";
import { useCallback, useEffect, useId, useState } from "react";
import { api, get, patch, post } from "@/lib/api";
import { useRequireSession } from "@/lib/auth";
import { invalidatePersonas } from "@/lib/personas";
import type { Character, CharacterInput, CharacterList } from "@/lib/types";

// Limits match the API's character validation (R-VALID).
const MAX_NAME = 80;
const MAX_ERA = 80;
const MAX_BIO = 300;
const MAX_PROMPT = 8000;

const AT_CAP = "Archive a character to create another";

// Characters is the reader's own character list: create, edit, archive and unarchive.
export function Characters() {
  const session = useRequireSession();
  const [list, setList] = useState<CharacterList>();
  const [creating, setCreating] = useState(false);
  const [error, setError] = useState<string>();

  const load = useCallback(() => {
    get<CharacterList>("/characters")
      .then((l) => {
        setList(l);
        setError(undefined);
      })
      .catch((e: Error) => setError(e.message));
  }, []);

  useEffect(() => {
    if (session) load();
  }, [session, load]);

  // changed runs after every successful mutation: the picker's cache and the
  // counts here are both stale now.
  const changed = useCallback(() => {
    invalidatePersonas();
    load();
  }, [load]);

  if (error && !list) return <p className="text-danger">{error}</p>;
  if (!session || !list) return <p className="text-muted">Loading…</p>;

  const atCap = list.active >= list.active_limit;
  const characters = list.characters ?? [];
  const active = characters.filter((c) => c.status === "active");
  const archived = characters.filter((c) => c.status === "archived");
  const removed = characters.filter((c) => c.status === "removed");

  async function create(input: CharacterInput) {
    await post<Character>("/characters", input);
    setCreating(false);
    changed();
  }

  return (
    <div className="space-y-8">
      <header className="space-y-1">
        <h1 className="font-serif text-3xl font-semibold">Your characters</h1>
        <p className="text-sm text-muted">
          {list.active} of {list.active_limit} active characters
        </p>
        {atCap && <p className="text-sm text-muted">{AT_CAP}</p>}
      </header>

      <section>
        {creating ? (
          <div className="card p-4">
            <h2 className="mb-3 font-semibold">New character</h2>
            <CharacterForm submitLabel="Create character" onSubmit={create} onCancel={() => setCreating(false)} />
          </div>
        ) : (
          <button
            className="btn btn-primary"
            disabled={atCap}
            title={atCap ? AT_CAP : undefined}
            onClick={() => setCreating(true)}
          >
            + New character
          </button>
        )}
      </section>

      {error && <p className="text-sm text-danger">{error}</p>}

      <CharacterSection title="Active" empty="No active characters yet. Create one above.">
        {active.map((c) => (
          <CharacterCard key={c.id} character={c} atCap={atCap} onChanged={changed} />
        ))}
      </CharacterSection>

      {archived.length > 0 && (
        <CharacterSection title="Archived">
          {archived.map((c) => (
            <CharacterCard key={c.id} character={c} atCap={atCap} onChanged={changed} />
          ))}
        </CharacterSection>
      )}

      {removed.length > 0 && (
        <CharacterSection title="Removed">
          {removed.map((c) => (
            <CharacterCard key={c.id} character={c} atCap={atCap} onChanged={changed} />
          ))}
        </CharacterSection>
      )}
    </div>
  );
}

// ---------------------------------------------------------------------------

function LivingPeopleNotice() {
  return (
    <p className="rounded-md bg-accent-soft px-3 py-2 text-sm text-accent">
      Historical or fictional figures only — no living people.{" "}
      <Link href="/terms" className="underline">
        Terms
      </Link>
    </p>
  );
}

function CharacterSection({
  title,
  empty,
  children,
}: {
  title: string;
  empty?: string;
  children: React.ReactNode[];
}) {
  return (
    <section className="space-y-3">
      <h2 className="font-semibold">{title}</h2>
      {children.length === 0 ? (
        <p className="text-sm text-muted">{empty}</p>
      ) : (
        <ul className="space-y-3">{children}</ul>
      )}
    </section>
  );
}

// ---------------------------------------------------------------------------

function CharacterCard({
  character: c,
  atCap,
  onChanged,
}: {
  character: Character;
  atCap: boolean;
  onChanged: () => void;
}) {
  const [editing, setEditing] = useState(false);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string>();

  async function setArchived(archive: boolean) {
    setBusy(true);
    setError(undefined);
    try {
      // No body: archive and unarchive take none.
      await api<Character>(`/characters/${encodeURIComponent(c.id)}/${archive ? "archive" : "unarchive"}`, {
        method: "POST",
      });
      onChanged();
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setBusy(false);
    }
  }

  async function save(input: CharacterInput) {
    // Send only what changed: omitted fields stay as they are. A locked name is never sent.
    const body: Partial<CharacterInput> = {};
    if (!c.name_locked && input.name !== c.name) body.name = input.name;
    if (input.era !== c.era) body.era = input.era;
    if (input.short_bio !== c.short_bio) body.short_bio = input.short_bio;
    if (input.prompt !== c.prompt) body.prompt = input.prompt;
    if (Object.keys(body).length > 0) {
      await patch<Character>(`/characters/${encodeURIComponent(c.id)}`, body);
      onChanged();
    }
    setEditing(false);
  }

  if (editing) {
    return (
      <li className="card p-4">
        <CharacterForm
          initial={c}
          nameLocked={c.name_locked}
          submitLabel="Save"
          onSubmit={save}
          onCancel={() => setEditing(false)}
        />
      </li>
    );
  }

  return (
    <li className="card space-y-2 p-4">
      <div>
        <p className="font-medium break-words">{c.name}</p>
        {c.era && <p className="text-sm text-muted break-words">{c.era}</p>}
      </div>
      {c.short_bio && <p className="text-sm break-words whitespace-pre-wrap">{c.short_bio}</p>}
      {c.status === "removed" ? (
        <p className="rounded-md border border-danger px-3 py-2 text-sm text-danger break-words">
          Removed: {c.removed_reason}
        </p>
      ) : (
        <div className="flex flex-wrap gap-2">
          <button className="btn" disabled={busy} onClick={() => setEditing(true)}>
            Edit
          </button>
          {c.status === "active" ? (
            <button className="btn" disabled={busy} onClick={() => setArchived(true)}>
              Archive
            </button>
          ) : (
            <button
              className="btn"
              disabled={busy || atCap}
              title={atCap ? AT_CAP : undefined}
              onClick={() => setArchived(false)}
            >
              Unarchive
            </button>
          )}
        </div>
      )}
      {error && <p className="text-sm text-danger">{error}</p>}
    </li>
  );
}

// ---------------------------------------------------------------------------

function CharacterForm({
  initial,
  nameLocked = false,
  submitLabel,
  onSubmit,
  onCancel,
}: {
  initial?: Character;
  nameLocked?: boolean;
  submitLabel: string;
  // onSubmit throws an API error to show inline.
  onSubmit: (input: CharacterInput) => Promise<void>;
  onCancel: () => void;
}) {
  const id = useId();
  const [name, setName] = useState(initial?.name ?? "");
  const [era, setEra] = useState(initial?.era ?? "");
  const [bio, setBio] = useState(initial?.short_bio ?? "");
  const [prompt, setPrompt] = useState(initial?.prompt ?? "");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string>();

  const valid = name.trim() !== "" && prompt.trim() !== "";

  async function submit(e: React.FormEvent) {
    e.preventDefault();
    if (!valid) return;
    setBusy(true);
    setError(undefined);
    try {
      await onSubmit({ name: name.trim(), era: era.trim(), short_bio: bio.trim(), prompt: prompt.trim() });
    } catch (err) {
      setError((err as Error).message);
      setBusy(false);
    }
  }

  return (
    <form onSubmit={submit} className="space-y-3">
      <LivingPeopleNotice />
      <div className="space-y-1">
        <label htmlFor={`${id}-name`} className="block text-sm font-medium">
          Name
        </label>
        <input
          id={`${id}-name`}
          value={name}
          onChange={(e) => setName(e.target.value)}
          maxLength={MAX_NAME}
          readOnly={nameLocked}
          required
          aria-describedby={nameLocked ? `${id}-name-hint` : undefined}
          className={`input ${nameLocked ? "cursor-not-allowed text-muted" : ""}`}
        />
        {nameLocked && (
          <p id={`${id}-name-hint`} className="text-xs text-muted">
            Locked: this character has been in a discussion
          </p>
        )}
      </div>

      <div className="space-y-1">
        <label htmlFor={`${id}-era`} className="block text-sm font-medium">
          Era <span className="font-normal text-muted">(optional)</span>
        </label>
        <input
          id={`${id}-era`}
          value={era}
          onChange={(e) => setEra(e.target.value)}
          maxLength={MAX_ERA}
          placeholder="e.g. Athens, 5th century BC"
          className="input"
        />
      </div>

      <div className="space-y-1">
        <div className="flex items-baseline justify-between gap-2">
          <label htmlFor={`${id}-bio`} className="block text-sm font-medium">
            Short bio <span className="font-normal text-muted">(optional)</span>
          </label>
          <Counter length={bio.length} max={MAX_BIO} />
        </div>
        <textarea
          id={`${id}-bio`}
          value={bio}
          onChange={(e) => setBio(e.target.value)}
          maxLength={MAX_BIO}
          rows={2}
          className="input resize-y"
        />
      </div>

      <div className="space-y-1">
        <div className="flex items-baseline justify-between gap-2">
          <label htmlFor={`${id}-prompt`} className="block text-sm font-medium">
            Prompt
          </label>
          <Counter length={prompt.length} max={MAX_PROMPT} />
        </div>
        <textarea
          id={`${id}-prompt`}
          value={prompt}
          onChange={(e) => setPrompt(e.target.value)}
          maxLength={MAX_PROMPT}
          rows={8}
          required
          placeholder="Who is this character, how do they talk, and what do they care about?"
          className="input resize-y text-[0.95rem]"
        />
      </div>

      <div className="flex flex-wrap gap-2">
        <button className="btn btn-primary" disabled={busy || !valid}>
          {submitLabel}
        </button>
        <button type="button" className="btn" disabled={busy} onClick={onCancel}>
          Cancel
        </button>
      </div>
      {error && <p className="text-sm text-danger">{error}</p>}
    </form>
  );
}

function Counter({ length, max }: { length: number; max: number }) {
  return (
    <span className={`text-xs tabular-nums ${length >= max ? "text-danger" : "text-muted"}`}>
      {length}/{max}
    </span>
  );
}
