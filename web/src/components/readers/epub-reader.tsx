"use client";

import ePub, { type Contents, type NavItem, type Rendition } from "epubjs";
import { useEffect, useRef } from "react";
import type { ReaderProps } from "./types";

function flatten(items: NavItem[]): NavItem[] {
  return items.flatMap((i) => [i, ...flatten(i.subitems ?? [])]);
}

function chapterLabel(toc: NavItem[], href: string): string | undefined {
  const base = href.split("#")[0];
  return toc.find((i) => {
    const h = i.href.split("#")[0];
    return h === base || base.endsWith("/" + h) || h.endsWith("/" + base);
  })?.label.trim();
}

export default function EpubReader({ data, initialLocation, jumpTo, onSelect, onProgress }: ReaderProps) {
  const container = useRef<HTMLDivElement>(null);
  const rendition = useRef<Rendition>(null);
  // Keep the latest callbacks without re-creating the book.
  const callbacks = useRef({ onSelect, onProgress });
  useEffect(() => {
    callbacks.current = { onSelect, onProgress };
  });

  useEffect(() => {
    if (!container.current) return;
    const book = ePub(data.slice(0));
    const r = book.renderTo(container.current, { width: "100%", height: "100%", flow: "paginated", spread: "none" });
    rendition.current = r;

    const css = getComputedStyle(document.body);
    r.themes.default({
      body: { color: css.color, background: css.backgroundColor, "font-family": "Literata, Georgia, serif" },
      "::selection": { background: "rgba(240, 145, 95, 0.35)" },
    });

    let toc: NavItem[] = [];
    let chapter: string | undefined;
    let locationsReady = false;

    book.loaded.navigation.then((nav) => (toc = flatten(nav.toc)));
    book.ready
      .then(() => book.locations.generate(1600))
      .then(() => {
        locationsReady = true;
        const loc = r.currentLocation() as unknown as { start?: { cfi: string } };
        if (loc?.start) report(loc.start.cfi);
      });

    const percentage = (cfi: string, fallback = 0) => {
      if (!locationsReady) return fallback;
      try {
        return book.locations.percentageFromCfi(cfi);
      } catch {
        return fallback;
      }
    };

    const report = (cfi: string, fallback = 0) => callbacks.current.onProgress(percentage(cfi, fallback), cfi);

    r.on("relocated", (loc: { start: { cfi: string; href: string; percentage: number } }) => {
      chapter = chapterLabel(toc, loc.start.href);
      report(loc.start.cfi, loc.start.percentage);
      callbacks.current.onSelect(null);
    });

    r.on("selected", (cfiRange: string, contents: Contents) => {
      // Read the live selection: a CFI built from an element-boundary range
      // (e.g. a triple-clicked paragraph) can map back to an empty range.
      const text = (contents.window.getSelection()?.toString() || r.getRange(cfiRange)?.toString() || "").trim();
      if (!text) return;
      const pct = percentage(cfiRange);
      const location = [chapter, locationsReady ? `${Math.round(pct * 100)}%` : undefined].filter(Boolean).join(" · ");
      callbacks.current.onSelect({ text, location, anchor: cfiRange, position: pct });
    });

    const onKey = (e: KeyboardEvent) => {
      if (e.key === "ArrowLeft") r.prev();
      if (e.key === "ArrowRight") r.next();
    };
    r.on("keyup", onKey);
    document.addEventListener("keyup", onKey);

    r.display(initialLocation ?? undefined);

    return () => {
      document.removeEventListener("keyup", onKey);
      r.destroy();
      book.destroy();
      rendition.current = null;
    };
    // initialLocation is only read on first render.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [data]);

  useEffect(() => {
    if (jumpTo && rendition.current) rendition.current.display(jumpTo);
  }, [jumpTo]);

  return (
    <div className="flex h-full flex-col">
      <div ref={container} className="min-h-0 flex-1" />
      <div className="flex justify-between border-t border-line pt-2">
        <button className="btn" onClick={() => rendition.current?.prev()}>
          ← Previous
        </button>
        <span className="self-center text-xs text-muted">Select text to quote it</span>
        <button className="btn" onClick={() => rendition.current?.next()}>
          Next →
        </button>
      </div>
    </div>
  );
}
