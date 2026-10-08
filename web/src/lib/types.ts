// Shapes returned by the Go API (api/internal/store).

export type Profile = {
  id: string;
  username: string;
  display_name: string;
  bio: string;
  is_public: boolean;
  created_at: string;
};

export type Work = {
  id: string;
  title: string;
  authors: string[];
  isbn: string | null;
  openlibrary_key: string | null;
  cover_url: string | null;
  subjects: string[];
};

export type SearchResult = {
  openlibrary_key: string;
  title: string;
  authors: string[];
  isbn: string | null;
  cover_url: string | null;
  subjects: string[];
  first_published: number | null;
};

export type FileType = "epub" | "pdf";
export type ReadingStatus = "reading" | "finished" | "abandoned";

export type LibraryItem = {
  id: string;
  work: Work;
  file_path: string | null;
  file_type: FileType | null;
  progress: number;
  status: ReadingStatus;
  last_location: string | null;
  quote_count: number;
  created_at: string;
  updated_at: string;
};

export type Visibility = "private" | "public";

export type Quote = {
  id: string;
  work_id: string;
  library_item_id: string | null;
  text: string;
  location: string;
  anchor: string | null;
  position: number | null;
  note: string;
  source: "upload" | "kindle";
  visibility: Visibility;
  created_at: string;
  updated_at: string;
  work?: Work;
};

export type Persona = {
  id: string;
  slug: string;
  name: string;
  era: string;
  short_bio: string;
  is_preset: boolean;
};

export type Mode = "react" | "argue";

export type Message = {
  id: number;
  role: "user" | "assistant";
  content: string;
  created_at: string;
};

export type Conversation = {
  id: string;
  quote_id: string;
  mode: Mode;
  persona: Persona;
  messages: Message[];
  created_at: string;
};

export type QuoteWithConversations = Quote & { conversations: Conversation[] };

export type PublicProfile = {
  profile: Profile;
  categories: { name: string; books: number }[];
  books: { work: Work; status: ReadingStatus; progress: number; public_quotes: number }[];
  quotes: QuoteWithConversations[];
};

export type KindleImportResult = {
  books: number;
  imported: number;
  duplicates: number;
  skipped_too_long: number;
  orphan_notes: number;
};

export type StreamEvent =
  | { type: "start"; conversation_id: string }
  | { type: "delta"; text: string }
  | { type: "done"; message: Message }
  | { type: "error"; error: string };
