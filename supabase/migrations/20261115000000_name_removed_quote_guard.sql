-- Name the trigger guard that keeps removed quotes out of public reads.
--
-- enforce_public_coverage raised its "a removed quote cannot be made public"
-- check violation without a constraint name, so the API could only recognise
-- it by the error's CONTEXT line. That line is localised by lc_messages and
-- breaks if the function is renamed (code review CR-012). The exception now
-- carries constraint 'quotes_removed_not_public', which the API matches on.
-- The function is otherwise unchanged.

create or replace function public.enforce_public_coverage() returns trigger
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
    raise exception 'a removed quote cannot be made public'
      using errcode = 'check_violation', constraint = 'quotes_removed_not_public';
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
