"use client";

import { createContext, useContext } from "react";
import { useSession } from "@/lib/auth";

// The signed-in viewer's user id: undefined while loading, null when signed out.
const ViewerContext = createContext<string | null | undefined>(undefined);

// ViewerProvider reads the session once for everything below it, so that a
// page with many viewer-dependent controls (one report button per quote and
// discussion) holds a single auth subscription instead of one per control.
export function ViewerProvider({ children }: { children: React.ReactNode }) {
  const session = useSession();
  const viewerId = session === undefined ? undefined : (session?.user.id ?? null);
  return <ViewerContext.Provider value={viewerId}>{children}</ViewerContext.Provider>;
}

// useViewerId returns the viewer's user id from the nearest ViewerProvider:
// undefined while loading or outside a provider, null when signed out.
export function useViewerId(): string | null | undefined {
  return useContext(ViewerContext);
}
