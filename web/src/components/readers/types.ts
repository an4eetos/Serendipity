// A passage the reader selected, ready to become a quote.
export type Selection = {
  text: string;
  location: string; // human-readable citation
  anchor: string; // where to jump back to (EPUB CFI or "page:N")
  position: number; // 0–1 through the book
};

export type ReaderProps = {
  data: ArrayBuffer;
  initialLocation: string | null;
  // Set to jump to a quote's anchor.
  jumpTo?: string;
  onSelect: (s: Selection | null) => void;
  onProgress: (progress: number, location: string) => void;
};
