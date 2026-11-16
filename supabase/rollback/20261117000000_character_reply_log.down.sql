-- Rollback for the character reply log: undoes migration
--   20261117000000_character_reply_log.sql
--
-- This is NOT a migration: the Supabase CLI never runs it. Run it by hand,
-- only after redeploying an API without the reply limit (it would fail every
-- character reply without the table):
--
--   psql "$DATABASE_URL" -v ON_ERROR_STOP=1 -f supabase/rollback/20261117000000_character_reply_log.down.sql
--
-- What it loses: the last hour of reply counts, so readers start a fresh
-- hour. Run it before supabase/rollback/20261116000000_phase2_custom_characters.down.sql.

begin;

drop table public.character_replies;

delete from supabase_migrations.schema_migrations where version = '20261117000000';

commit;
