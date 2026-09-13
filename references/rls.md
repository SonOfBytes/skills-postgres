# Row-Level Security

RLS is defence in depth for shared-table multi-tenancy: the explicit tenant parameter in every
query is the first fence (`modelling.md`); a policy is the second, so a forgotten predicate
returns nothing rather than everything. It is not a substitute for grants, and it is not a total
information barrier.

## Enable it correctly

- `ENABLE ROW LEVEL SECURITY` **and** `FORCE ROW LEVEL SECURITY` on every scoped table. Table
  owners bypass RLS unless forced, and the application connecting as the owner is the most
  common way a policy silently does nothing [PG ddl-rowsecurity].
- The application role has neither `SUPERUSER` nor `BYPASSRLS`; both always bypass [PG
  ddl-rowsecurity].
- Grants still apply first: a role with no `SELECT` grant sees nothing regardless of policy; a
  role with a grant and no matching policy also sees nothing (default deny once RLS is on)
  [Supabase RLS].

## Set the tenant per transaction

`SET LOCAL`, never `SET`: the value must die with the transaction, because the next checkout of
a pooled connection is another tenant. Read it with the two-argument `current_setting(name,
true)`, which returns NULL rather than erroring when unset [Crunchy RLS for tenants].

```sql
ALTER TABLE video_blocks ENABLE ROW LEVEL SECURITY;
ALTER TABLE video_blocks FORCE ROW LEVEL SECURITY;

CREATE POLICY video_blocks_tenant ON video_blocks
  AS PERMISSIVE
  FOR ALL
  TO app_rw
  USING (household_id = (SELECT NULLIF(current_setting('app.household_id', true), '')::uuid))
  WITH CHECK (household_id = (SELECT NULLIF(current_setting('app.household_id', true), '')::uuid));
```

```sql
BEGIN;
SET LOCAL app.household_id = '3f1c2a44-6a0b-4d5e-9b1a-2c7e8f9a0b1c';
SELECT video_id FROM video_blocks;
COMMIT;
```

## Write policies that perform

- Wrap function calls in a scalar subquery `(SELECT current_setting(…))` so the planner
  evaluates them once per statement as an initPlan, not once per row (179 ms to 9 ms in
  Supabase's measurement) [Supabase RLS performance].
- Index every column a policy filters on; a policy is a `WHERE` clause evaluated per candidate
  row (over 100x in Supabase's test) [Supabase RLS performance].
- Name the role: `TO app_rw`. Without it the policy is considered for every role; with it,
  evaluation short-circuits [Supabase RLS].
- Membership checks go `tenant_id IN (SELECT tenant_id FROM memberships WHERE user_id = …)`,
  fetching the set once, not a correlated join against each row (9,000 ms to 20 ms) [Supabase
  RLS performance].
- Break policy recursion (policy on A reads B whose policy reads A) with a `SECURITY DEFINER`
  helper in a non-exposed schema, search path pinned (`security.md`) [Supabase RLS].

## Semantics that surprise

- Permissive policies **OR** together: adding one widens access. Policies that must all hold are
  `AS RESTRICTIVE` [PG ddl-rowsecurity].
- `USING` alone applies the same expression as `WITH CHECK`; state `WITH CHECK` deliberately so
  an insert cannot create a row the tenant could not then read [PG ddl-rowsecurity].
- Foreign-key, unique and primary-key checks bypass RLS: a constraint error can reveal that
  another tenant's row exists. Map 23503 and 23505 to a neutral message [PG ddl-rowsecurity].
- Never put a user-modifiable claim (a JWT's user-editable metadata) in a predicate [Supabase
  RLS].
- Backups and ETL run with `SET row_security = off` so accidental filtering raises an error
  instead of silently producing a partial dump [PG ddl-rowsecurity].

## Test it as the application role

A policy test run as the owner or superuser passes while production leaks. Connect as `app_rw`,
set tenant A, assert none of tenant B's rows appear for each of `SELECT`, `INSERT`, `UPDATE`,
`DELETE`; then remove the policy and watch the test fail once (`testing.md`) [Supabase RLS; PG
ddl-rowsecurity].

## Sources

- https://www.postgresql.org/docs/current/ddl-rowsecurity.html
- https://supabase.com/docs/guides/database/postgres/row-level-security
- https://www.crunchydata.com/blog/row-level-security-for-tenants-in-postgres
- https://supabase.com/docs/guides/troubleshooting/rls-performance-and-best-practices-Z5Jjwv
