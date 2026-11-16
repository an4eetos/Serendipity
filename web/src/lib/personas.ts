"use client";

import { get } from "./api";
import type { Persona } from "./types";

let personasCache: Promise<Persona[]> | undefined;

// loadPersonas returns the characters the reader can pick: presets, then their
// own active characters. The result is cached until invalidatePersonas.
export function loadPersonas(): Promise<Persona[]> {
  personasCache ??= get<Persona[]>("/personas").catch((e) => {
    personasCache = undefined;
    throw e;
  });
  return personasCache;
}

// invalidatePersonas drops the cache, so the next loadPersonas refetches. Call
// it after creating, editing, archiving or unarchiving a character.
export function invalidatePersonas(): void {
  personasCache = undefined;
}
