# Migrations: Process, Locks and Tools

A migration is code that runs once against every environment in order, under locks, with no
undo. The per-statement safe sequences are in `migration-recipes.md`; this file is the physics
and the process.

## When the table is not live

Everything below about `CONCURRENTLY`, `NOT VALID` and lock timeouts exists to keep a table
readable and writable **while the migration runs**. If the deployment migrates before it serves
(one instance, the migration gated by a deployment-only flag, no traffic until it completes), the
table is not live, and the plain forms are correct and simpler: a `CREATE INDEX` inside the
migration's transaction, a `NOT NULL` in one statement. Say which case you are in; the project's
operations document usually states it. The recipes become mandatory the day a second replica or
zero-downtime deploys arrive.

## Lock physics

- Assume every `ALTER TABLE` takes `ACCESS EXCLUSIVE` unless you have checked the form; it is the
  only lock mode that blocks a plain `SELECT` [PG explicit-locking].
- Forms that take less: `VALIDATE CONSTRAINT`, `SET STATISTICS`, storage-parameter changes
  (`SHARE UPDATE EXCLUSIVE`); `ADD FOREIGN KEY` (`SHARE ROW EXCLUSIVE` on both tables); `ATTACH
  PARTITION` (`SHARE UPDATE EXCLUSIVE` on the parent) [PG sql-altertable].
- A DDL statement waiting for `ACCESS EXCLUSIVE` queues, and **every later query queues behind
  it**, readers included: a blocked migration takes the table offline. Therefore every migration
  session sets `lock_timeout` and the runner retries with backoff and jitter [postgres.ai].

```sql
SET LOCAL lock_timeout = '2s';
ALTER TABLE videos ADD COLUMN kind text NOT NULL DEFAULT 'video';
```

`SET LOCAL` when the file runs inside a transaction (golang-migrate, goose and Flyway wrap a
file by default); plain `SET` is fine on a dedicated migrator connection that closes afterwards.
Either beats leaving it unset. A `SET` in a migration file is one of the legitimate uses of
session `SET`; the rule against it (`transactions.md`) is about pooled application connections.

- Roles: the migrator has a short `lock_timeout` (seconds) and a long `statement_timeout`
  (an hour, so a legitimate `VALIDATE` finishes); the application role the reverse
  [strong_migrations].

```sql
ALTER ROLE migrator SET lock_timeout = '5s';
ALTER ROLE migrator SET statement_timeout = '1h';
ALTER ROLE app SET lock_timeout = '2s';
ALTER ROLE app SET statement_timeout = '15s';
```

- One migration touching several tables acquires all its locks explicitly at the top (`LOCK
  TABLE a, b IN ACCESS EXCLUSIVE MODE` under the timeout), or is split; GitLab allows one foreign
  key per transaction [postgres.ai; GitLab style guide].
- Several rewriting subcommands on the **same** table go in one `ALTER TABLE`, so the table is
  rewritten once [PG sql-altertable].

## Transactions and the exceptions

Each migration runs in a transaction, so a failed step leaves nothing half-applied. Three things
cannot run inside one and get their own single-statement file: `CREATE INDEX CONCURRENTLY`,
`REINDEX CONCURRENTLY`, `VACUUM` [PG sql-createindex]. Tell the tool: golang-migrate runs one
file as one transaction (so the concurrent index is alone in its file); goose `-- +goose NO
TRANSACTION`; Atlas `-- atlas:txmode none`; Flyway detects it; EF Core
`suppressTransaction: true` on `migrationBuilder.Sql`.

## Process

- **One change class per migration.** Not two tables, not DDL plus a data change: locks are
  held until commit, so combining multiplies duration and blast radius [GitLab style guide].
- **Time limits.** GitLab's are a usable default: a regular migration under three minutes, a
  post-deployment migration under ten, a concurrent index under twenty; anything longer becomes
  a batched background job [GitLab style guide].
- **Applied migrations are immutable.** Flyway stores a checksum and fails `validate` when an
  applied file changes; tools without checksums (golang-migrate) need the rule as policy. A
  documented exemption ("the first migration may be edited before the first deployment, and the
  exemption ends at that deployment") is legitimate; write it down [Flyway validate].
- **Migrate as a release step, not on every application start.** N replicas starting together
  race on the migration lock; the release pipeline runs the migration once, then rolls the
  application out. A start-up migration gated by a flag that only the deployment sets, on a
  single instance, *is* a release step; the problem is unconditional migration on every boot of
  every replica. The application validates the schema at start or in readiness (below) [GitLab
  style guide].
- **Forward-only, or tested downs.** If down files exist, CI runs `up`, `down 1`, `up` on the
  newest pair; otherwise declare forward-only and roll forward with a compensating migration.
  Untested downs are the ones that fail at 3 a.m. [goose README].
- **Lint in CI.** squawk (`--pg-version 16.0 --assume-in-transaction`) or Atlas's analyzers
  catch missing `CONCURRENTLY`, missing `NOT VALID`, `timestamp` without zone, `serial`, and
  dropped columns before review does [squawk rules; Atlas analyzers].
- **Schema-qualify** objects so the result does not depend on `search_path` [squawk rules].
- Backfills run **outside** the DDL transaction in bounded batches with a pause, and are
  idempotent and resumable (a `WHERE new_col IS NULL` predicate or a stored high-water mark),
  so a killed backfill restarts without harm [strong_migrations; GitLab style guide].

## The schema guard

The application refuses to serve a schema it does not carry. On start (or in readiness) it reads
the migration tool's version table directly and compares with the highest migration embedded in
the binary:

| State | Action |
|---|---|
| dirty (a migration failed part way) | fatal, with the remedy command in the message |
| database behind the binary | fatal: refusing to start beats querying columns that do not exist |
| database ahead of the binary | warn: safe while the newer migrations were additive, which the migrate-then-roll-out order guarantees |
| no version table | version 0: a fresh database is a normal case |

Read the table directly (`SELECT version, dirty FROM schema_migrations`) because the migration
library refuses to open a dirty schema. The classifier is a pure function with a table test.

## Tools

| Tool | Model | Note |
|---|---|---|
| golang-migrate | versioned SQL, `.up.sql`/`.down.sql` pairs | one file = one transaction; no `lock_timeout` of its own; dirty flag on failure; `pgx5://` scheme with pgx v5 |
| goose | versioned SQL or Go, `-- +goose Up/Down` | `Down` optional (forward-only is explicit); `NO TRANSACTION` annotation |
| tern | versioned SQL, Postgres-only | minimal; pairs with pgx |
| Atlas | declarative or versioned | built-in linting with stable analyzer codes; computed rollbacks |
| sqitch | change-based with dependencies | deploy/revert/verify scripts |
| Flyway | versioned SQL `V1__name.sql` | checksum validation; rollback in the paid tier |
| Liquibase | changesets | broad database coverage; free rollback |
| EF Core migrations | model-diff generated C# | emits non-concurrent indexes and blocking ALTERs; review the generated SQL (`drivers/npgsql.md`) |
| pgroll | expand/contract via versioned views | old and new application versions read different schemas at once; `[pg>=14]` |
| squawk | linter | drop-in on any of the above |

Choose the one the language's ecosystem already uses; the discipline above matters more than the
tool. The driver-specific notes are in `drivers/`.

## Sources

- https://www.postgresql.org/docs/current/explicit-locking.html
- https://www.postgresql.org/docs/current/sql-altertable.html
- https://postgres.ai/blog/20210923-zero-downtime-postgres-schema-migrations-lock-timeout-and-retries
- https://github.com/ankane/strong_migrations
- https://docs.gitlab.com/development/migration_style_guide/
- https://www.postgresql.org/docs/current/sql-createindex.html
- https://documentation.red-gate.com/flyway/reference/commands/validate
- https://github.com/pressly/goose
- https://squawkhq.com/docs/rules
- https://atlasgo.io/lint/analyzers
- https://xataio.github.io/pgroll/
- https://atlasgo.io/concepts/declarative-vs-versioned
