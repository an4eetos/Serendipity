"use client";

import { useEffect, useRef, useState } from "react";
import { Document, Page, pdfjs } from "react-pdf";
import "react-pdf/dist/Page/AnnotationLayer.css";
import "react-pdf/dist/Page/TextLayer.css";
import type { ReaderProps } from "./types";

// Must be set in the module that renders react-pdf components.
pdfjs.GlobalWorkerOptions.workerSrc = new URL("pdfjs-dist/build/pdf.worker.min.mjs", import.meta.url).toString();

function pageFrom(anchor: string | null | undefined): number | undefined {
  const m = anchor?.match(/^page:(\d+)$/);
  return m ? Number(m[1]) : undefined;
}

// react-pdf reloads whenever the `file` object changes and suspends while
// loading. A useMemo'd object doesn't survive renders that never commit, so a
// fresh object each retry would reload forever; cache it per buffer instead.
// It gets a copy because react-pdf transfers the bytes to its worker.
const files = new WeakMap<ArrayBuffer, { data: Uint8Array }>();
function fileFor(data: ArrayBuffer) {
  let f = files.get(data);
  if (!f) {
    f = { data: new Uint8Array(data.slice(0)) };
    files.set(data, f);
  }
  return f;
}

export default function PdfReader({ data, initialLocation, jumpTo, onSelect, onProgress }: ReaderProps) {
  const file = fileFor(data);
  const [numPages, setNumPages] = useState(0);
  const [page, setPage] = useState(() => pageFrom(initialLocation) ?? 1);
  const [width, setWidth] = useState(600);
  const box = useRef<HTMLDivElement>(null);

  // Jump when a different quote is opened (state adjusted during render).
  const [lastJump, setLastJump] = useState(jumpTo);
  if (jumpTo !== lastJump) {
    setLastJump(jumpTo);
    const p = pageFrom(jumpTo);
    if (p) setPage(p);
  }

  useEffect(() => {
    if (!box.current) return;
    // The box reserves its scrollbar gutter, so the page appearing taller or
    // shorter than the box can't change this width and loop.
    const ro = new ResizeObserver(([e]) => setWidth(Math.floor(Math.min(900, e.contentRect.width))));
    ro.observe(box.current);
    return () => ro.disconnect();
  }, []);

  useEffect(() => {
    if (numPages) onProgress(page / numPages, `page:${page}`);
    onSelect(null);
    // Report only when the page changes.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [page, numPages]);

  function handleMouseUp() {
    const text = window.getSelection()?.toString().trim();
    if (!text) return;
    onSelect({ text, location: `Page ${page}`, anchor: `page:${page}`, position: numPages ? page / numPages : 0 });
  }

  const go = (p: number) => setPage(Math.max(1, Math.min(numPages || 1, p)));

  return (
    <div className="flex h-full flex-col">
      <div ref={box} className="min-h-0 flex-1 overflow-auto [scrollbar-gutter:stable]" onMouseUp={handleMouseUp}>
        <Document
          file={file}
          onLoadSuccess={({ numPages }) => {
            setNumPages(numPages);
            setPage((p) => Math.min(p, numPages));
          }}
          loading={<p className="text-sm text-muted">Opening PDF…</p>}
          error={<p className="text-sm text-danger">Could not open this PDF.</p>}
        >
          <Page pageNumber={page} width={width} />
        </Document>
      </div>
      <div className="flex justify-between border-t border-line pt-2">
        <button className="btn" disabled={page <= 1} onClick={() => go(page - 1)}>
          ← Previous
        </button>
        <span className="self-center text-xs text-muted">
          Page {page} of {numPages || "…"} · select text to quote it
        </span>
        <button className="btn" disabled={page >= numPages} onClick={() => go(page + 1)}>
          Next →
        </button>
      </div>
    </div>
  );
}
