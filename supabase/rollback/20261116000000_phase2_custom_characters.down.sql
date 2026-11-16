-- Rollback for Phase 2, "Custom characters": undoes migration
--   20261116000000_phase2_custom_characters.sql
-- and returns the schema to the Phase 1 state (20261115000000).
--
-- This is NOT a migration: the Supabase CLI never runs it. Run it by hand,
-- only after redeploying the Phase 1 API and web (they don't know archived or
-- removed characters), and only after a backup:
--
--   psql "$DATABASE_URL" -v ON_ERROR_STOP=1 -f supabase/rollback/20261116000000_phase2_custom_characters.down.sql
--
-- To go further back, run supabase/rollback/20261024000000_phase1_open_to_strangers.down.sql
-- afterwards; it refuses to run while Phase 2 is applied.
--
-- What it loses, by design: the archive state (archived characters come back
-- to the picker), personas.updated_at, the "depicts a living person" report
-- reason (those reports become "other", with the reason kept as a prefix of
-- their note), and open reports on characters, which are dismissed because
-- the Phase 1 API can neither show nor dismiss them.
--
-- Taken-down content must not come back with the rollback: the Phase 1 schema
-- can't hide a character, so the script stops if a removed character exists.
-- Decide first: restore the character, or delete its conversations and the
-- character, then run the script again.
--
-- Everything runs in one transaction: on any error nothing changes.

begin;

do $$
begin
  if exists (select 1 from public.personas where removed_at is not null) then
    raise exception 'removed characters exist: restore them, or delete their conversations and the characters, before rolling back';
  end if;
end $$;

-- Reports: character reports, then the Phase 2 reason.
update public.reports
  set status = 'dismissed', resolved_at = now(), updated_at = now()
  where target_kind = 'character' and status = 'open';

update public.reports
  set note = left('[Depicts a living person] ' || note, 1000), reason = 'other'
  where reason = 'living_person';

-- Postgres can't drop an enum value: recreate the type without it.
alter type public.report_reason rename to report_reason_phase2;
create type public.report_reason as enum ('copyright', 'harassment', 'hate', 'spam', 'other');
alter table public.reports
  alter column reason type public.report_reason using reason::text::public.report_reason;
drop type public.report_reason_phase2;

-- Guards: triggers before their functions.
drop trigger messages_guard_persona on public.messages;
drop trigger conversations_guard_persona on public.conversations;
drop trigger personas_guard_name on public.personas;
drop function public.guard_message_persona();
drop function public.guard_conversation_persona();
drop function public.check_persona_usable(uuid);
drop function public.guard_persona_name();

-- Indexes, then the character state columns and checks.
drop index public.conversations_persona_idx;
drop index public.personas_owner_idx;
alter table public.personas
  drop constraint personas_name_not_blank,
  drop constraint personas_era_length,
  drop constraint personas_prompt_not_blank,
  drop constraint personas_presets_active,
  drop constraint personas_removed_reason,
  drop column archived_at,
  drop column removed_at,
  drop column removed_reason,
  drop column updated_at;

-- Forget the migration, so a later `supabase db push` applies it again.
delete from supabase_migrations.schema_migrations where version = '20261116000000';

commit;
