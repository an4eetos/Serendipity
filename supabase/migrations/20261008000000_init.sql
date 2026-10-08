-- Serendipity: initial schema.
--
-- Copyright model: books uploaded by readers live in the private `books`
-- storage bucket and are readable only by the uploader. Only quotes (short,
-- cited, with the reader's own note) and the conversations around them can be
-- made public.

-- ---------------------------------------------------------------------------
-- Types
-- ---------------------------------------------------------------------------

create type public.file_type as enum ('epub', 'pdf');
create type public.reading_status as enum ('reading', 'finished', 'abandoned');
create type public.visibility as enum ('private', 'public');
create type public.quote_source as enum ('upload', 'kindle');
create type public.conversation_mode as enum ('react', 'argue');
create type public.message_role as enum ('user', 'assistant');

-- ---------------------------------------------------------------------------
-- Profiles
-- ---------------------------------------------------------------------------

create table public.profiles (
  id           uuid primary key references auth.users (id) on delete cascade,
  username     text not null unique check (username ~ '^[a-z0-9_]{3,30}$'),
  display_name text not null default '' check (char_length(display_name) <= 80),
  bio          text not null default '' check (char_length(bio) <= 500),
  is_public    boolean not null default false,
  created_at   timestamptz not null default now()
);

-- Every new auth user gets a profile with a username derived from their email.
create function public.handle_new_user() returns trigger
language plpgsql security definer set search_path = '' as $$
declare
  base      text;
  candidate text;
  n         int := 0;
begin
  base := lower(regexp_replace(split_part(coalesce(new.email, ''), '@', 1), '[^a-zA-Z0-9_]', '', 'g'));
  if char_length(base) < 3 then
    base := 'reader';
  end if;
  base := left(base, 24);
  candidate := base;
  while exists (select 1 from public.profiles where username = candidate) loop
    n := n + 1;
    candidate := base || '_' || n;
  end loop;
  insert into public.profiles (id, username, display_name) values (new.id, candidate, base);
  return new;
end $$;

create trigger on_auth_user_created
  after insert on auth.users
  for each row execute function public.handle_new_user();

-- ---------------------------------------------------------------------------
-- Works (catalog metadata only — never the text itself)
-- ---------------------------------------------------------------------------

create table public.works (
  id              uuid primary key default gen_random_uuid(),
  title           text not null check (char_length(title) between 1 and 500),
  authors         text[] not null default '{}',
  isbn            text,
  openlibrary_key text unique,
  cover_url       text,
  subjects        text[] not null default '{}',
  created_by      uuid references auth.users (id) on delete set null,
  created_at      timestamptz not null default now()
);

-- ---------------------------------------------------------------------------
-- Library: a reader's copy of a work (optionally with a private file)
-- ---------------------------------------------------------------------------

create table public.library_items (
  id            uuid primary key default gen_random_uuid(),
  user_id       uuid not null references public.profiles (id) on delete cascade,
  work_id       uuid not null references public.works (id) on delete restrict,
  file_path     text,
  file_type     public.file_type,
  progress      double precision not null default 0 check (progress between 0 and 1),
  status        public.reading_status not null default 'reading',
  last_location text,
  created_at    timestamptz not null default now(),
  updated_at    timestamptz not null default now(),
  unique (user_id, work_id),
  check ((file_path is null) = (file_type is null))
);

-- ---------------------------------------------------------------------------
-- Quotes
-- ---------------------------------------------------------------------------

create table public.quotes (
  id              uuid primary key default gen_random_uuid(),
  user_id         uuid not null references public.profiles (id) on delete cascade,
  work_id         uuid not null references public.works (id) on delete restrict,
  library_item_id uuid references public.library_items (id) on delete set null,
  text            text not null check (char_length(text) between 1 and 3000),
  -- Human-readable citation, e.g. "Chapter 3 · 41%" or "Page 12 · Loc 170-172".
  location        text not null default '' check (char_length(location) <= 200),
  -- Machine anchor to jump back into the reader (EPUB CFI or "page:N").
  anchor          text,
  -- How far into the book the quote is (0–1), for the reading-trail map.
  position        double precision check (position between 0 and 1),
  note            text not null default '' check (char_length(note) <= 5000),
  source          public.quote_source not null,
  visibility      public.visibility not null default 'private',
  created_at      timestamptz not null default now(),
  updated_at      timestamptz not null default now(),
  -- A public quote must be commentary with a citation, not bare copying.
  constraint public_quote_needs_note_and_citation check (
    visibility = 'private'
    or (char_length(btrim(note)) > 0 and char_length(btrim(location)) > 0)
  )
);

create index quotes_user_work_idx on public.quotes (user_id, work_id);
create index quotes_public_work_idx on public.quotes (work_id) where visibility = 'public';
-- Re-importing the same Kindle clippings must not duplicate quotes.
create unique index quotes_dedupe_idx on public.quotes (user_id, work_id, md5(text));

-- ---------------------------------------------------------------------------
-- Personas (AI characters)
-- ---------------------------------------------------------------------------

create table public.personas (
  id            uuid primary key default gen_random_uuid(),
  slug          text not null unique check (slug ~ '^[a-z0-9-]{2,60}$'),
  name          text not null check (char_length(name) between 1 and 80),
  era           text not null default '',
  short_bio     text not null default '' check (char_length(short_bio) <= 300),
  system_prompt text not null check (char_length(system_prompt) <= 8000),
  is_preset     boolean not null default false,
  owner_id      uuid references public.profiles (id) on delete cascade,
  sort_order    int not null default 0,
  created_at    timestamptz not null default now(),
  check (is_preset = (owner_id is null))
);

-- ---------------------------------------------------------------------------
-- Conversations about a quote. Visibility follows the quote.
-- ---------------------------------------------------------------------------

create table public.conversations (
  id         uuid primary key default gen_random_uuid(),
  user_id    uuid not null references public.profiles (id) on delete cascade,
  quote_id   uuid not null references public.quotes (id) on delete cascade,
  persona_id uuid not null references public.personas (id) on delete restrict,
  mode       public.conversation_mode not null default 'react',
  created_at timestamptz not null default now()
);

create index conversations_quote_idx on public.conversations (quote_id, created_at);

create table public.messages (
  id              bigint generated always as identity primary key,
  conversation_id uuid not null references public.conversations (id) on delete cascade,
  role            public.message_role not null,
  content         text not null check (char_length(content) between 1 and 20000),
  created_at      timestamptz not null default now()
);

create index messages_conversation_idx on public.messages (conversation_id, id);

-- ---------------------------------------------------------------------------
-- Row-level security. The Go API connects as a privileged role and enforces
-- ownership itself; these policies protect direct access via the Supabase
-- client (anon/authenticated keys).
-- ---------------------------------------------------------------------------

alter table public.profiles      enable row level security;
alter table public.works         enable row level security;
alter table public.library_items enable row level security;
alter table public.quotes        enable row level security;
alter table public.personas      enable row level security;
alter table public.conversations enable row level security;
alter table public.messages      enable row level security;

create function public.is_public_profile(uid uuid) returns boolean
language sql stable security definer set search_path = '' as $$
  select coalesce((select is_public from public.profiles where id = uid), false)
$$;

create policy "profiles: readable if public or own" on public.profiles
  for select using (is_public or id = auth.uid());
create policy "profiles: own update" on public.profiles
  for update using (id = auth.uid()) with check (id = auth.uid());

create policy "works: readable by everyone" on public.works
  for select using (true);

create policy "library: own" on public.library_items
  for all using (user_id = auth.uid()) with check (user_id = auth.uid());
create policy "library: readable on public profiles" on public.library_items
  for select using (public.is_public_profile(user_id));

create policy "quotes: own" on public.quotes
  for all using (user_id = auth.uid()) with check (user_id = auth.uid());
create policy "quotes: public ones on public profiles" on public.quotes
  for select using (visibility = 'public' and public.is_public_profile(user_id));

create policy "personas: presets readable" on public.personas
  for select using (is_preset);
create policy "personas: own" on public.personas
  for all using (owner_id = auth.uid()) with check (owner_id = auth.uid());

create policy "conversations: own" on public.conversations
  for all using (user_id = auth.uid()) with check (user_id = auth.uid());
create policy "conversations: on public quotes" on public.conversations
  for select using (exists (
    select 1 from public.quotes q
    where q.id = quote_id and q.visibility = 'public' and public.is_public_profile(q.user_id)
  ));

create policy "messages: own" on public.messages
  for all using (exists (
    select 1 from public.conversations c where c.id = conversation_id and c.user_id = auth.uid()
  )) with check (exists (
    select 1 from public.conversations c where c.id = conversation_id and c.user_id = auth.uid()
  ));
create policy "messages: on public quotes" on public.messages
  for select using (exists (
    select 1 from public.conversations c
    join public.quotes q on q.id = c.quote_id
    where c.id = conversation_id and q.visibility = 'public' and public.is_public_profile(q.user_id)
  ));

-- ---------------------------------------------------------------------------
-- Storage: private bucket for readers' own copies, path "<user id>/<file>".
-- ---------------------------------------------------------------------------

insert into storage.buckets (id, name, public, file_size_limit, allowed_mime_types)
values ('books', 'books', false, 52428800, array['application/epub+zip', 'application/pdf']);

create policy "books: owner reads" on storage.objects
  for select to authenticated
  using (bucket_id = 'books' and (storage.foldername(name))[1] = auth.uid()::text);
create policy "books: owner uploads" on storage.objects
  for insert to authenticated
  with check (bucket_id = 'books' and (storage.foldername(name))[1] = auth.uid()::text);
create policy "books: owner updates" on storage.objects
  for update to authenticated
  using (bucket_id = 'books' and (storage.foldername(name))[1] = auth.uid()::text);
create policy "books: owner deletes" on storage.objects
  for delete to authenticated
  using (bucket_id = 'books' and (storage.foldername(name))[1] = auth.uid()::text);
