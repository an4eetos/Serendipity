-- Rollback for Phase 1, "Open to strangers": undoes migrations
--   20261024000000_phase1_open_to_strangers.sql
--   20261115000000_name_removed_quote_guard.sql
-- and returns the schema to 20261008000000_init.sql.
--
-- Roll Phase 2 back first: this script refuses to run while
-- supabase/rollback/20261116000000_phase2_custom_characters.down.sql hasn't.
--
-- This is NOT a migration: the Supabase CLI never runs it. Run it by hand,
-- only after redeploying the Phase 0 API and web (they don't know the
-- moderation tables or the terms gate), and only after a backup:
--
--   psql "$DATABASE_URL" -v ON_ERROR_STOP=1 -f supabase/rollback/20261024000000_phase1_open_to_strangers.down.sql
--
-- What it loses, by design: terms acceptances, ownership confirmations,
-- reports, takedowns, the admins list, per-work cap overrides and the
-- collapsed state of passages over the cap.
--
-- Taken-down content must not come back with the rollback:
--   * removed quotes are made private (their owners still see them);
--   * a removed discussion can't be hidden on its own in the Phase 0 schema,
--     so the script stops if one exists. Decide first: delete it, or make its
--     quote private, then run the script again.
--
-- Everything runs in one transaction: on any error nothing changes.

begin;

do $$
begin
  if exists (
    select 1 from information_schema.columns
    where table_schema = 'public' and table_name = 'personas' and column_name = 'archived_at'
  ) then
    raise exception 'Phase 2 is applied: run supabase/rollback/20261116000000_phase2_custom_characters.down.sql first';
  end if;
end $$;

do $$
begin
  if exists (select 1 from public.conversations where removed_at is not null) then
    raise exception 'removed discussions exist: delete them or make their quotes private before rolling back';
  end if;
end $$;

update public.quotes set visibility = 'private' where removed_at is not null and visibility = 'public';

-- Coverage trigger (both migrations define its function).
drop trigger quotes_enforce_public_coverage on public.quotes;
drop function public.enforce_public_coverage();

-- Moderation.
drop table public.takedowns;
drop table public.reports;
drop table public.admins;
drop type public.report_status;
drop type public.report_reason;

-- RLS: back to the Phase 0 policies, which let clients write their own rows
-- and read public profiles directly.
create function public.is_public_profile(uid uuid) returns boolean
language sql stable security definer set search_path = '' as $$
  select coalesce((select is_public from public.profiles where id = uid), false)
$$;

drop policy "profiles: own read" on public.profiles;
create policy "profiles: readable if public or own" on public.profiles
  for select using (is_public or id = auth.uid());
create policy "profiles: own update" on public.profiles
  for update using (id = auth.uid()) with check (id = auth.uid());

drop policy "library: own read" on public.library_items;
create policy "library: own" on public.library_items
  for all using (user_id = auth.uid()) with check (user_id = auth.uid());
create policy "library: readable on public profiles" on public.library_items
  for select using (public.is_public_profile(user_id));

drop policy "quotes: own read" on public.quotes;
create policy "quotes: own" on public.quotes
  for all using (user_id = auth.uid()) with check (user_id = auth.uid());
create policy "quotes: public ones on public profiles" on public.quotes
  for select using (visibility = 'public' and public.is_public_profile(user_id));

drop policy "personas: own read" on public.personas;
create policy "personas: own" on public.personas
  for all using (owner_id = auth.uid()) with check (owner_id = auth.uid());

drop policy "conversations: own read" on public.conversations;
create policy "conversations: own" on public.conversations
  for all using (user_id = auth.uid()) with check (user_id = auth.uid());
create policy "conversations: on public quotes" on public.conversations
  for select using (exists (
    select 1 from public.quotes q
    where q.id = quote_id and q.visibility = 'public' and public.is_public_profile(q.user_id)
  ));

drop policy "messages: own read" on public.messages;
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

-- Conversations: removal.
alter table public.conversations
  drop column removed_at,
  drop column removed_reason;

-- Quotes: coverage columns and removal. The generated columns go before the
-- functions they call.
drop index public.quotes_coverage_idx;
alter table public.quotes
  drop column passage_hash,
  drop column passage_words,
  drop column passage_hidden,
  drop column removed_at,
  drop column removed_reason;
create index quotes_public_work_idx on public.quotes (work_id) where visibility = 'public';

drop function public.passage_word_count(text);
drop function public.passage_hash(text);
drop function public.normalize_passage(text);

-- Per-work cap, ownership confirmation, terms.
alter table public.works drop column public_word_cap;
alter table public.library_items drop column ownership_confirmed_at;
alter table public.profiles
  drop constraint profiles_terms_accepted_together,
  drop column terms_accepted_at,
  drop column terms_version;

-- Forget both migrations, so a later `supabase db push` applies them again.
delete from supabase_migrations.schema_migrations where version in ('20261024000000', '20261115000000');

commit;
