// Shapes returned by the Go API (api/internal/store).

export type Profile = {
  id: string;
  username: string;
  display_name: string;
  bio: string;
  is_public: boolean;
  created_at: string;
};

export type Me = Profile & {
  terms_version: string;
  terms_accepted_at: string | null;
  current_terms_version: string;
  terms_accepted: boolean;
};

export type Terms = { version: string };

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
  ownership_confirmed_at: string | null;
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
  reaction: string;
  source: "upload" | "kindle";
  visibility: Visibility;
  passage_hidden: boolean;
  removed_at: string | null;
  removed_reason: string;
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

export type Message = {
  id: number;
  role: "user" | "assistant";
  content: string;
  created_at: string;
};

export type Conversation = {
  id: string;
  quote_id: string;
  persona: Persona;
  messages: Message[];
  removed_at: string | null;
  removed_reason: string;
  created_at: string;
};

export type QuoteWithConversations = Quote & { conversations: Conversation[] };

export type TrailPoint = { quote_id: string; position: number };

export type PublicBook = {
  work: Work;
  status: ReadingStatus;
  progress: number;
  public_quotes: number;
  trail: { points: TrailPoint[]; unplaced: number };
};

export type PublicProfile = {
  profile: Profile;
  categories: { name: string; books: number }[];
  books: PublicBook[];
  quotes: QuoteWithConversations[];
};

export type ReportTargetKind = "quote" | "conversation";
export type ReportReason = "copyright" | "harassment" | "hate" | "spam" | "other";
export type ReportStatus = "open" | "actioned" | "dismissed";

export const reportReasonLabels: Record<ReportReason, string> = {
  copyright: "Copyright infringement",
  harassment: "Harassment",
  hate: "Hate speech",
  spam: "Spam",
  other: "Something else",
};

export type Report = {
  id: string;
  target_kind: ReportTargetKind;
  target_id: string;
  reason: ReportReason;
  note: string;
  status: ReportStatus;
  created_at: string;
};

export type ReportEntry = { id: string; reporter_id: string; reason: ReportReason; note: string; created_at: string };

export type ReportedTarget = {
  target_kind: ReportTargetKind;
  target_id: string;
  owner_id: string;
  owner_username: string;
  quote: Quote | null;
  conversation: Conversation | null;
  reports: ReportEntry[];
};

export type Takedown = {
  id: string;
  target_kind: ReportTargetKind;
  target_id: string;
  owner_id: string;
  owner_username: string;
  reason: string;
  hidden_by: string | null;
  created_at: string;
  restored_at: string | null;
  restored_by: string | null;
};

export type TakedownWithTarget = Takedown & { quote: Quote | null; conversation: Conversation | null };

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
