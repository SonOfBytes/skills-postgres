# Constraints: Keys, Foreign Keys, CHECK, Unique, Exclusion

Invariants that are true forever belong in the database. Rules that change belong in the
application. The line: "a user account password not being null is a constraint; passwords
between 6 and 14 characters is policy" [PG ddl-constraints].

## Primary keys

- Every table has an explicit primary key: it is what logical replication, ORMs and foreign keys
  need, and `pg_dump` cannot restore a duplicate-free table without one [PG ddl-constraints;
  Supabase tables].
- Surrogate `bigint` identity plus a `UNIQUE` on the natural key when the natural key is wide
  or may ever change; a natural primary key only when it is immutable and narrow, because it is
  copied into every referencing index [PG ddl-constraints].
- Junction tables get a composite primary key `(left_id, right_id)`; a surrogate on a junction
  table buys nothing and permits duplicates [PG ddl-constraints].
- In a tenant-scoped table the tenant column is the **first** column of the primary key and of
  every scoped unique constraint, so one index serves both the constraint and the per-tenant
  lookup (see `modelling.md`).

```sql
CREATE TABLE channel_approvals (
  household_id uuid NOT NULL REFERENCES households (id) ON DELETE CASCADE,
  channel_id   uuid NOT NULL REFERENCES channels (id) ON DELETE CASCADE,
  approved_at  timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (household_id, channel_id)
);
```

## Foreign keys

- Declare one for every reference. Application-level integrity is not integrity: a crash between
  two writes leaves an orphan the application will never see again [GitLab foreign keys].
- Every foreign key states `ON DELETE`. The default `NO ACTION` pushes deletion orchestration
  into application code. Choose deliberately [GitLab foreign keys; PG ddl-constraints; Brandur
  idempotency keys]:

| Relationship | Action | Why |
|---|---|---|
| Owned child rows (line items, approvals, progress) | `CASCADE` | the child has no meaning without the parent |
| Audit, log, history or idempotency rows that must outlive the parent | `SET NULL` | the record survives, the pointer clears |
| A reference that must block deletion until resolved | `RESTRICT` | immediate, not deferrable |
| A reference checked at the end of a deferred transaction | `NO ACTION DEFERRABLE INITIALLY DEFERRED` | only for genuine circular writes |

- Index the referencing column unless the child table is small, the parent key is never deleted
  or updated, and you never join on it: Postgres does not create the index, and every parent
  `DELETE` otherwise scans the child (154 ms against 0.75 ms in Cybertec's test) [PG
  ddl-constraints; Cybertec FK index]. Create the index before the constraint on a live table
  (see `migration-recipes.md`).
- Composite foreign keys: `MATCH FULL`, or make every referencing column `NOT NULL`; under the
  default match a row with one NULL component escapes the constraint. `ON DELETE SET NULL
  (column_list)` nulls only the listed columns, so a composite child key survives [PG
  ddl-constraints].
- Deferrable constraints only for the circular case, with a comment; they delay error detection
  and hold row locks longer [PG ddl-constraints].

```sql
CREATE TABLE video_watch_state (
  user_id         uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
  video_id        uuid NOT NULL REFERENCES videos (id) ON DELETE CASCADE,
  last_watched_at timestamptz NOT NULL,
  completed       boolean NOT NULL DEFAULT false,
  PRIMARY KEY (user_id, video_id)
);
CREATE INDEX video_watch_state_video_idx ON video_watch_state (video_id);
```

## CHECK

- A `CHECK` is a row-local invariant: it cannot reference other rows or tables, it is evaluated
  only when its row is inserted or updated, and it **passes when the expression is NULL**. A
  user-defined function inside a `CHECK` is unsafe because a change to the function is not
  detected [PG ddl-constraints].
- Cross-column invariants are its natural home: "a playlist track has a playlist id and a channel
  track does not" is one expression.

```sql
CREATE TABLE tracks (
  id                  uuid PRIMARY KEY,
  kind                text NOT NULL CHECK (kind IN ('channel', 'playlist')),
  youtube_playlist_id text,
  CHECK ((kind = 'channel') = (youtube_playlist_id IS NULL))
);
```

- Domains carry a `CHECK` once for many tables, but `CREATE DOMAIN … CHECK` and `ALTER DOMAIN
  ADD CONSTRAINT` revalidate every dependent table under heavy locks; squawk bans both in
  migrations for that reason [squawk rules]. Prefer a domain only for a type used widely from the
  start.

## Unique

- Conditional uniqueness is a partial unique index: "one open invite per address", "one ready
  asset per video". Promote it to a constraint with `ADD CONSTRAINT … UNIQUE USING INDEX` when a
  tool needs to see it as one [PG sql-altertable; pganalyze create-index].
- A partial unique index cannot be deferred, so a state swap that must pass through the
  constraint runs as two statements in one transaction, old row leaving the state before the new
  one enters it; write the comment that says why the order matters.
- `[pg>=15]` `UNIQUE NULLS NOT DISTINCT` when NULL should collide; by default two NULLs are
  distinct and a nullable unique column permits unlimited duplicates [PG ddl-constraints].
- Case-insensitive uniqueness: a unique index on `lower(email)` (or the `citext` extension when
  the whole column is case-insensitive).

```sql
CREATE UNIQUE INDEX household_invites_open_idx
  ON household_invites (lower(email))
  WHERE claimed_at IS NULL;

CREATE UNIQUE INDEX media_assets_one_ready_idx
  ON media_assets (video_id)
  WHERE state = 'ready';
```

## Exclusion

- Non-overlap invariants (bookings, validity periods, leases) are exclusion constraints over a
  range type with GiST; a `SELECT`-then-`INSERT` overlap check races under concurrency [PG
  ddl-constraints; PG rangetypes]. `btree_gist` supplies the `=` operator for the scalar part.

```sql
CREATE EXTENSION IF NOT EXISTS btree_gist;

CREATE TABLE reservations (
  id      bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  room_id bigint NOT NULL REFERENCES rooms (id) ON DELETE CASCADE,
  during  tstzrange NOT NULL,
  EXCLUDE USING gist (room_id WITH =, during WITH &&)
);
```

- `[pg>=18]` temporal keys express the same thing natively: `PRIMARY KEY (room_id, during
  WITHOUT OVERLAPS)` and `FOREIGN KEY (room_id, PERIOD during) REFERENCES rooms (id, PERIOD
  valid)` [PG 18 release notes].

## Naming

Every constraint on a table that will be migrated has an explicit name; the auto-generated one
differs between environments and makes `DROP CONSTRAINT` non-deterministic [Atlas analyzers].
See `naming.md` for the scheme.

## Adding constraints to live tables

Every form above has a lock-safe sequence (`NOT VALID` then `VALIDATE`, unique via a concurrent
index, `NOT NULL` in four steps). See `migration-recipes.md`; do not write a bare `ADD
CONSTRAINT` on a table with rows.

## Sources

- https://www.postgresql.org/docs/current/ddl-constraints.html
- https://supabase.com/docs/guides/database/tables
- https://docs.gitlab.com/development/database/foreign_keys/
- https://brandur.org/idempotency-keys
- https://www.cybertec-postgresql.com/en/index-your-foreign-key/
- https://squawkhq.com/docs/rules
- https://www.postgresql.org/docs/current/sql-altertable.html
- https://pganalyze.com/blog/postgres-create-index
- https://www.postgresql.org/docs/current/rangetypes.html
- https://www.postgresql.org/docs/release/18.0/
- https://atlasgo.io/lint/analyzers
