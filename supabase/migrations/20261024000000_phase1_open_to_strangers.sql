-- Phase 1, "Open to strangers": terms of service, ownership confirmation,
-- moderation (reports, takedowns, admins), the public-coverage cap per work
-- and RLS hardening. Every statement is transactional.

-- ---------------------------------------------------------------------------
-- 1. Terms of service (R1.2)
-- ---------------------------------------------------------------------------

alter table public.profiles
  add column terms_version     text not null default '' check (char_length(terms_version) <= 64),
  add column terms_accepted_at timestamptz,
  add constraint profiles_terms_accepted_together check ((terms_version = '') = (terms_accepted_at is null));

-- ---------------------------------------------------------------------------
-- 2. Ownership confirmation (R1.4, R1.5): the latest time the reader
--    confirmed they own the copy they uploaded or imported from.
-- ---------------------------------------------------------------------------

alter table public.library_items add column ownership_confirmed_at timestamptz;

-- ---------------------------------------------------------------------------
-- 3. Per-work cap override (R3.2), set by SQL only
-- ---------------------------------------------------------------------------

alter table public.works
  add column public_word_cap int check (public_word_cap is null or public_word_cap >= 0);

-- ---------------------------------------------------------------------------
-- 4. Passage normalization, shared by the generated columns and the trigger:
--    lower-case, collapse whitespace runs to one space, trim.
-- ---------------------------------------------------------------------------

create function public.normalize_passage(t text) returns text
language sql immutable parallel safe set search_path = '' as $$
  select pg_catalog.btrim(pg_catalog.regexp_replace(pg_catalog.lower(t), '\s+', ' ', 'g'))
$$;

create function public.passage_hash(t text) returns text
language sql immutable parallel safe set search_path = '' as $$
  select pg_catalog.md5(public.normalize_passage(t))
$$;

create function public.passage_word_count(t text) returns int
language sql immutable parallel safe set search_path = '' as $$
  select coalesce(pg_catalog.array_length(
    pg_catalog.string_to_array(nullif(public.normalize_passage(t), ''), ' '), 1), 0)
$$;

-- ---------------------------------------------------------------------------
-- 5. Quotes: coverage columns + removal (R2, R3)
-- ---------------------------------------------------------------------------

alter table public.quotes
  add column passage_hash   text generated always as (public.passage_hash(text)) stored,
  add column passage_words  int  generated always as (public.passage_word_count(text)) stored,
  add column passage_hidden boolean not null default false,
  add column removed_at     timestamptz,
  add column removed_reason text not null default '' check (char_length(removed_reason) <= 500);

-- Replaced by the coverage index (nothing reads quotes by work_id alone).
drop index public.quotes_public_work_idx;
create index quotes_coverage_idx on public.quotes (work_id, passage_hash) include (passage_words)
  where visibility = 'public' and removed_at is null and not passage_hidden;

-- ---------------------------------------------------------------------------
-- 6. Conversations: removal (R2)
-- ---------------------------------------------------------------------------

alter table public.conversations
  add column removed_at     timestamptz,
  add column removed_reason text not null default '' check (char_length(removed_reason) <= 500);

-- ---------------------------------------------------------------------------
-- 7. Coverage trigger (R3.3). Generated columns are NULL in BEFORE triggers,
--    so NEW's hash and word count are computed from NEW.text here.
--    security definer: it must lock works and read every reader's quotes.
--    The default cap comes from the transaction-local setting
--    serendipity.public_word_cap, which the API sets with set_config(…, true)
--    before every write that can move a quote into coverage (D9). Otherwise 2500.
-- ---------------------------------------------------------------------------

create function public.enforce_public_coverage() returns trigger
language plpgsql security definer set search_path = '' as $$
declare
  new_hash  text := public.passage_hash(new.text);
  new_words int  := public.passage_word_count(new.text);
  word_cap  int;
  covered   int;
begin
  -- A removed quote stays out of public reads until a moderator restores it.
  if new.removed_at is not null and new.visibility = 'public'
     and (tg_op = 'INSERT' or old.visibility <> 'public') then
    raise exception 'a removed quote cannot be made public' using errcode = 'check_violation';
  end if;

  -- Only a passage that starts counting toward coverage is checked.
  if new.visibility <> 'public' or new.removed_at is not null or new.passage_hidden then
    return new;
  end if;
  if tg_op = 'UPDATE' and old.visibility = 'public' and old.removed_at is null
     and not old.passage_hidden and old.work_id = new.work_id and old.text = new.text then
    return new;
  end if;

  -- Same lock the API takes in UpdateQuote (re-entrant within one transaction).
  perform 1 from public.works w where w.id = new.work_id for no key update;

  -- An identical passage that is already shown counts once.
  if exists (
    select 1 from public.quotes q
    where q.work_id = new.work_id and q.id <> new.id and q.passage_hash = new_hash
      and q.visibility = 'public' and q.removed_at is null and not q.passage_hidden
  ) then
    return new;
  end if;

  select coalesce(w.public_word_cap,
                  nullif(pg_catalog.current_setting('serendipity.public_word_cap', true), '')::int,
                  2500)
    into word_cap
    from public.works w where w.id = new.work_id;

  select coalesce(sum(d.words), 0) into covered from (
    select max(q.passage_words) as words
    from public.quotes q
    where q.work_id = new.work_id and q.id <> new.id
      and q.visibility = 'public' and q.removed_at is null and not q.passage_hidden
    group by q.passage_hash
  ) d;

  if covered + new_words > word_cap then
    new.passage_hidden := true;
  end if;
  return new;
end $$;

create trigger quotes_enforce_public_coverage
  before insert or update on public.quotes
  for each row execute function public.enforce_public_coverage();

-- ---------------------------------------------------------------------------
-- 8. Moderation (R2.3, R2.7, R5.2)
-- ---------------------------------------------------------------------------

create type public.report_reason as enum ('copyright', 'harassment', 'hate', 'spam', 'other');
create type public.report_status as enum ('open', 'actioned', 'dismissed');

create table public.admins (
  user_id    uuid primary key references auth.users (id) on delete cascade,
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now()
);

create table public.reports (
  id              uuid primary key default gen_random_uuid(),
  reporter_id     uuid not null references public.profiles (id) on delete cascade,
  target_kind     text not null check (target_kind ~ '^[a-z][a-z_]{0,31}$'),
  target_id       text not null check (char_length(target_id) between 1 and 64),
  target_owner_id uuid not null references public.profiles (id) on delete cascade,
  reason          public.report_reason not null,
  note            text not null default '' check (char_length(note) <= 1000),
  status          public.report_status not null default 'open',
  resolved_by     uuid references auth.users (id) on delete set null,
  resolved_at     timestamptz,
  created_at      timestamptz not null default now(),
  updated_at      timestamptz not null default now(),
  constraint reports_not_own_content check (reporter_id <> target_owner_id),
  constraint reports_resolved_when_closed check ((status = 'open') = (resolved_at is null))
);

-- One open report per reader per target (R2.1).
create unique index reports_one_open_idx on public.reports (reporter_id, target_kind, target_id)
  where status = 'open';
create index reports_open_target_idx on public.reports (target_kind, target_id) where status = 'open';
create index reports_open_created_idx on public.reports (created_at) where status = 'open';

create table public.takedowns (
  id              uuid primary key default gen_random_uuid(),
  target_kind     text not null check (target_kind ~ '^[a-z][a-z_]{0,31}$'),
  target_id       text not null check (char_length(target_id) between 1 and 64),
  target_owner_id uuid not null references public.profiles (id) on delete cascade,
  reason          text not null check (char_length(btrim(reason)) between 1 and 500),
  hidden_by       uuid references auth.users (id) on delete set null,
  restored_by     uuid references auth.users (id) on delete set null,
  restored_at     timestamptz,
  created_at      timestamptz not null default now(),
  updated_at      timestamptz not null default now()
);

-- At most one active takedown per target.
create unique index takedowns_one_active_idx on public.takedowns (target_kind, target_id)
  where restored_at is null;
create index takedowns_active_created_idx on public.takedowns (created_at desc)
  where restored_at is null;

alter table public.admins    enable row level security;
alter table public.reports   enable row level security;
alter table public.takedowns enable row level security;
-- No policies: only the API's privileged role reads or writes these.
revoke all on table public.admins, public.reports, public.takedowns from anon, authenticated;

-- Admin bootstrap (operator, SQL editor):
--   insert into public.admins (user_id) values ('<auth user uuid>');

-- ---------------------------------------------------------------------------
-- 9. RLS hardening (R5.1): the Go API (privileged role) is the only writer and
--    the only cross-user reader. Clients keep owner-read, works, preset
--    personas and every storage.objects policy for the books bucket.
-- ---------------------------------------------------------------------------

drop policy "profiles: readable if public or own" on public.profiles;
drop policy "profiles: own update"                on public.profiles;
create policy "profiles: own read" on public.profiles
  for select to authenticated using (id = auth.uid());

drop policy "library: own"                         on public.library_items;
drop policy "library: readable on public profiles" on public.library_items;
create policy "library: own read" on public.library_items
  for select to authenticated using (user_id = auth.uid());

drop policy "quotes: own"                            on public.quotes;
drop policy "quotes: public ones on public profiles" on public.quotes;
create policy "quotes: own read" on public.quotes
  for select to authenticated using (user_id = auth.uid());

drop policy "personas: own" on public.personas;
create policy "personas: own read" on public.personas
  for select to authenticated using (owner_id = auth.uid());
-- kept: "personas: presets readable", "works: readable by everyone"

drop policy "conversations: own"              on public.conversations;
drop policy "conversations: on public quotes" on public.conversations;
create policy "conversations: own read" on public.conversations
  for select to authenticated using (user_id = auth.uid());

drop policy "messages: own"              on public.messages;
drop policy "messages: on public quotes" on public.messages;
create policy "messages: own read" on public.messages
  for select to authenticated using (exists (
    select 1 from public.conversations c where c.id = conversation_id and c.user_id = auth.uid()
  ));

drop function public.is_public_profile(uuid);
