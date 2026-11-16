-- Character reply log: one row per character reply a reader asks for, so the
-- API can cap replies per reader per hour (CHARACTER_REPLIES_PER_HOUR). Every
-- reply is a paid model call.
--
-- The log is separate from messages on purpose: deleting a discussion must
-- not give the reader their replies back. The API prunes a reader's rows older
-- than the window each time it adds one, so the table stays small.

create table public.character_replies (
  id         bigint generated always as identity primary key,
  user_id    uuid not null references public.profiles (id) on delete cascade,
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now()
);

-- Counting and pruning one reader's recent rows.
create index character_replies_user_idx on public.character_replies (user_id, created_at);

-- Only the API's privileged role reads or writes it.
alter table public.character_replies enable row level security;
revoke all on table public.character_replies from anon, authenticated;
