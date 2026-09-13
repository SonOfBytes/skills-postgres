# Migration Recipes: The Safe Sequence for Each Change

Each recipe names the unsafe form, the lock it takes, and the sequence that keeps the table
online. All assume `SET LOCAL lock_timeout` at the top of the file (`migrations.md`). Version
markers say where a newer major shortens the recipe. If the table is not live during the
migration (`migrations.md`, "When the table is not live"), the plain form is the right one and
the recipe is a note for later.

## Add a column

- With a non-volatile default: one statement, no rewrite (since 11); the default is stored in
  the catalog and applied on read [PG sql-altertable].
- With a volatile default (`now()`, `gen_random_uuid()`): the naive form rewrites the table.
  Add nullable, set the default for new rows, backfill in batches, then `NOT NULL` by the recipe
  below [strong_migrations].

```sql
ALTER TABLE videos ADD COLUMN first_seen_at timestamptz;
ALTER TABLE videos ALTER COLUMN first_seen_at SET DEFAULT now();
```

```sql
-- Batched, idempotent, resumable backfill; run outside the DDL transaction, with a pause.
UPDATE videos SET first_seen_at = created_at
WHERE id IN (
  SELECT id FROM videos WHERE first_seen_at IS NULL LIMIT 10000
);
```

## Add a nullable column with a CHECK

No rewrite: a nullable column without a default is a catalog change. The inline `CHECK` is
verified against existing rows under the same `ACCESS EXCLUSIVE` lock; every existing row is
NULL, so the scan passes and is brief on a small table. On a large live table add the `CHECK`
`NOT VALID` and validate separately (below) [PG sql-altertable].

```sql
ALTER TABLE channel_approvals
  ADD COLUMN pin_position smallint
    CONSTRAINT check_channel_approvals_pin_position CHECK (pin_position BETWEEN 1 AND 3);
```

## Add an index on a live table

`CREATE INDEX` takes `SHARE` and blocks writes for the whole build. `CONCURRENTLY` takes `SHARE
UPDATE EXCLUSIVE`, does two scans, waits for old transactions, and cannot run inside a
transaction, so it gets its own single-statement file. A failure leaves an `INVALID` index that
is maintained on writes and never used; drop it and retry [PG sql-createindex; squawk].

```sql
CREATE INDEX CONCURRENTLY IF NOT EXISTS videos_need_meta_idx
  ON videos (metadata_attempts, first_seen_at)
  WHERE metadata_fetched_at IS NULL;
```

## Add a foreign key or CHECK

`ADD CONSTRAINT` scans the table under a heavy lock. `NOT VALID` records the constraint for new
rows immediately; `VALIDATE CONSTRAINT` in a later migration scans under `SHARE UPDATE
EXCLUSIVE`, which permits reads and writes [PG sql-altertable; squawk]. Create the supporting
index on the child column (concurrently) **before** the foreign key [GitLab foreign keys].

```sql
ALTER TABLE track_items
  ADD CONSTRAINT fk_track_items_video_id_videos
  FOREIGN KEY (video_id) REFERENCES videos (id) ON DELETE CASCADE NOT VALID;
```

```sql
ALTER TABLE track_items VALIDATE CONSTRAINT fk_track_items_video_id_videos;
```

## Add NOT NULL

A bare `SET NOT NULL` scans under `ACCESS EXCLUSIVE`. Four steps: backfill; add a `CHECK (col IS
NOT NULL) NOT VALID`; `VALIDATE` it; `SET NOT NULL`, which skips the scan because a valid proving
`CHECK` exists; then drop the `CHECK` [GitLab NOT NULL guide; PG sql-altertable]. `[pg>=18]` a
named `NOT NULL … NOT VALID` then `VALIDATE` does it in two [PG 18 release notes].

```sql
ALTER TABLE videos
  ADD CONSTRAINT check_videos_first_seen_at_not_null
  CHECK (first_seen_at IS NOT NULL) NOT VALID;
```

```sql
ALTER TABLE videos VALIDATE CONSTRAINT check_videos_first_seen_at_not_null;
ALTER TABLE videos ALTER COLUMN first_seen_at SET NOT NULL;
ALTER TABLE videos DROP CONSTRAINT check_videos_first_seen_at_not_null;
```

## Add a unique constraint

`ADD CONSTRAINT … UNIQUE` builds its index non-concurrently. Build the unique index
concurrently, then promote it [PG sql-altertable; strong_migrations].

```sql
CREATE UNIQUE INDEX CONCURRENTLY users_subject_uidx ON users (subject);
```

```sql
ALTER TABLE users ADD CONSTRAINT unique_users_subject UNIQUE USING INDEX users_subject_uidx;
```

## Change a column's type

`ALTER COLUMN … TYPE` rewrites the table under `ACCESS EXCLUSIVE` except in the no-rewrite cases:
`varchar(n)` to `text` or a wider `varchar`, `timestamp` to `timestamptz` when the session zone is
UTC, `numeric` precision increased at the same scale, `cidr` to `inet` [strong_migrations; PG
sql-altertable]. Otherwise expand/contract: add the new column, dual-write from the application,
backfill in batches, switch reads, stop writing the old column, drop it two releases later.

## `int` to `bigint` primary key

The in-place change rewrites the table and every referencing index. Crunchy's sequence: add a
`bigint` column with its own identity, dual-write, backfill, then in one short transaction swap
the primary key and rename [Crunchy serials should be bigint].

## Rename a column or table

A rename breaks every running instance that uses the old name; Atlas classes it
backward-incompatible (BC101, BC102). Expand/contract as for a type change, or a view with the
old name for the transition [strong_migrations; Atlas analyzers].

## Drop a column

Two deploys: first ship code that no longer references the column (and, for ORMs that cache the
column list, explicitly ignores it), then drop [strong_migrations].

```sql
ALTER TABLE videos DROP COLUMN legacy_flag;
```

## Add a STORED generated column

On a large live table it rewrites. `[pg>=18]` add it `VIRTUAL` (no storage, computed on read) or
add a plain column and backfill [strong_migrations; PG ddl-generated-columns].

## Change an enum

Adding a value is one statement (`ALTER TYPE … ADD VALUE`, cannot run inside a transaction on
older majors). Renaming or removing a value is not supported in place: add the new value, teach
the application to accept both, backfill, stop writing the old one [strong_migrations].

## Partition operations

`[pg>=14]` `DETACH PARTITION … CONCURRENTLY`; `ATTACH` after adding a matching `CHECK` so the
attach skips validation; `[pg>=18]` `ANALYZE ONLY parent` [PG sql-altertable].

## Rebuild a bloated index

`REINDEX INDEX … CONCURRENTLY` (since 12) takes `SHARE UPDATE EXCLUSIVE`; on failure a
`_ccnew` index remains (drop and retry) or `_ccold` (the rebuild succeeded, drop it). It cannot
rebuild an exclusion-constraint index [PG sql-reindex].

## Backfill in batches

Outside the DDL transaction, ~10,000 rows per statement with a short sleep between, an
idempotent predicate so the job can be rerun, and GitLab's rule that no single batch query
exceeds one second on a cold cache [strong_migrations; GitLab style guide].

## Sources

- https://www.postgresql.org/docs/current/sql-altertable.html
- https://github.com/ankane/strong_migrations
- https://www.postgresql.org/docs/current/sql-createindex.html
- https://squawkhq.com/docs/rules
- https://docs.gitlab.com/development/database/foreign_keys/
- https://docs.gitlab.com/development/database/not_null_constraints/
- https://www.postgresql.org/docs/release/18.0/
- https://www.crunchydata.com/blog/postgres-serials-should-be-bigint-and-how-to-migrate
- https://atlasgo.io/lint/analyzers
- https://www.postgresql.org/docs/current/ddl-generated-columns.html
- https://www.postgresql.org/docs/current/sql-reindex.html
- https://docs.gitlab.com/development/migration_style_guide/
