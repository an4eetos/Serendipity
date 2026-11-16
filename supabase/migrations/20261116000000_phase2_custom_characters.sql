-- Phase 2, "Custom characters": readers' own characters can be archived,
-- are name-locked once used, and can be reported and taken down.
--
-- `alter type ... add value` may run inside the migration's transaction
-- (Postgres 12+), but the new value can't be USED until that transaction
-- commits: nothing below may reference the new report reason.

-- ---------------------------------------------------------------------------
-- 1. Report reason (R6)
-- ---------------------------------------------------------------------------

alter type public.report_reason add value if not exists 'living_person';

-- ---------------------------------------------------------------------------
-- 2. Character state (R1-R3, R7, D10)
-- ---------------------------------------------------------------------------

alter table public.personas
  add column archived_at    timestamptz,
  add column removed_at     timestamptz,
  add column removed_reason text not null default '' check (char_length(removed_reason) <= 500),
  add column updated_at     timestamptz not null default now(),
  add constraint personas_name_not_blank   check (btrim(name) <> ''),
  add constraint personas_era_length       check (char_length(era) <= 80),
  add constraint personas_prompt_not_blank check (btrim(system_prompt) <> ''),
  add constraint personas_presets_active   check (not is_preset or (archived_at is null and removed_at is null)),
  add constraint personas_removed_reason   check ((removed_at is null) = (removed_reason = ''));

-- Owner's list and active count; name lock and FK restrict checks.
create index personas_owner_idx on public.personas (owner_id, created_at desc) where owner_id is not null;
create index conversations_persona_idx on public.conversations (persona_id);

-- ---------------------------------------------------------------------------
-- 3. Name lock (R2, D6): the name is fixed once any conversation uses it.
--    Runs after the UPDATE has locked the row; the exists() gets a fresh
--    snapshot (volatile plpgsql, READ COMMITTED), so it sees a conversation a
--    concurrent transaction committed while we waited.
-- ---------------------------------------------------------------------------

create function public.guard_persona_name() returns trigger
language plpgsql security definer set search_path = '' as $$
begin
  if new.name is distinct from old.name
     and exists (select 1 from public.conversations c where c.persona_id = old.id) then
    raise exception 'a character''s name can''t change once it has been in a discussion'
      using errcode = 'check_violation', constraint = 'personas_name_locked';
  end if;
  return new;
end $$;

create trigger personas_guard_name
  before update of name on public.personas
  for each row execute function public.guard_persona_name();

-- ---------------------------------------------------------------------------
-- 4. Only active characters take part in discussions (R3, R7, D3, D5).
--    FOR SHARE conflicts with the FOR NO KEY UPDATE an archive, removal or
--    edit takes, so the insert and the state change serialize.
-- ---------------------------------------------------------------------------

create function public.check_persona_usable(target uuid) returns void
language plpgsql security definer set search_path = '' as $$
declare
  archived timestamptz;
  removed  timestamptz;
begin
  select p.archived_at, p.removed_at into archived, removed
    from public.personas p where p.id = target for share;
  if removed is not null then
    raise exception 'this character was removed by a moderator'
      using errcode = 'check_violation', constraint = 'personas_removed';
  end if;
  if archived is not null then
    raise exception 'this character is archived'
      using errcode = 'check_violation', constraint = 'personas_archived';
  end if;
end $$;
revoke execute on function public.check_persona_usable(uuid) from public, anon, authenticated;

create function public.guard_conversation_persona() returns trigger
language plpgsql security definer set search_path = '' as $$
begin
  perform public.check_persona_usable(new.persona_id);
  return new;
end $$;

create trigger conversations_guard_persona
  before insert on public.conversations
  for each row execute function public.guard_conversation_persona();

-- D13: a removed conversation takes no new message either. FOR SHARE on the
-- conversation row conflicts with the FOR NO KEY UPDATE that
-- ConversationModeration.Remove's UPDATE takes, so the two serialize.
create function public.guard_message_persona() returns trigger
language plpgsql security definer set search_path = '' as $$
declare
  persona      uuid;
  conv_removed timestamptz;
begin
  select c.persona_id, c.removed_at into persona, conv_removed
    from public.conversations c where c.id = new.conversation_id for share;
  if conv_removed is not null then
    raise exception 'this discussion was removed by a moderator'
      using errcode = 'check_violation', constraint = 'conversations_removed';
  end if;
  perform public.check_persona_usable(persona);
  return new;
end $$;

create trigger messages_guard_persona
  before insert on public.messages
  for each row execute function public.guard_message_persona();

-- RLS: unchanged. "personas: own read" and "personas: presets readable" stay;
-- the API (privileged role) is the only writer. reports/takedowns need no
-- change: target_kind 'character' matches '^[a-z][a-z_]{0,31}$'.
