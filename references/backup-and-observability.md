# Backups, Recovery, Monitoring and Health

A database without a tested restore is a cache. A database without `pg_stat_statements` is a
black box. Both are cheap to fix on day one.

## Backups

- Any database with a real recovery objective has a **physical** backup with continuous WAL
  archiving (pgBackRest, WAL-G or barman): base backups plus every WAL segment, so you can
  restore to a point in time. `pg_dump` alone omits roles and tablespaces and cannot do PITR;
  where it is the chosen tool, pair it with `pg_dumpall --globals-only` [PG backup-dump;
  pgBackRest].
- A personal tool still needs a backup: a scheduled `pg_dump -Fc` to another host, with a
  restore you have run once. Do not deploy pgBackRest for one household; do not skip the dump.
- Encrypt the repository client-side (`repo1-cipher-type=aes-256-cbc`), so the storage provider
  never sees plaintext; define retention explicitly (`repo1-retention-full` at least 2) and
  dry-run any change, because expiring a backup expires its WAL and shrinks the PITR window
  [pgBackRest].
- Test restores on a schedule, on a separate host, including a point-in-time target
  (`--type=time`); run `pgbackrest check` after any configuration change. An untested backup
  fails at the worst moment [pgBackRest; PG backup-dump].
- Restore into a database created from `template0`, then `ANALYZE` [PG backup-dump].
- Backups run with `row_security = off` where RLS exists (`rls.md`).

```sql
-- pg_dump restore target: created from template0 so the dump's settings are not doubled.
CREATE DATABASE kvs_restore TEMPLATE template0 ENCODING 'UTF8';
```

## Upgrades

`pg_upgrade --check` first; `--link` is fastest but irreversible once the new cluster starts;
`--clone` keeps the old cluster usable; logical replication for near-zero downtime. Afterwards
`vacuumdb --all --analyze-in-stages` (`[pg>=18]` statistics carry over) [PG pgupgrade].

## What to watch

| Signal | Where | Why |
|---|---|---|
| slow and frequent queries | `pg_stat_statements` (`total_exec_time`, `calls`, `mean_exec_time`); query ids change across majors | the primary performance signal [PG pgstatstatements] |
| what sessions are doing | `pg_stat_activity` by `state` and `wait_event_type`, not raw count | `Lock`, `ClientRead` and `idle in transaction` are different incidents [PG monitoring-stats] |
| bloat and stale plans | `pg_stat_user_tables`: `n_dead_tup`, `n_mod_since_analyze`, `last_autovacuum`, `seq_scan` vs `idx_scan` | leading indicators [PG monitoring-stats] |
| I/O attribution | `[pg>=16]` `pg_stat_io` by backend type and context | vacuum I/O no longer pollutes the cache-hit figure [PG monitoring-stats] |
| wraparound | `age(datfrozenxid)` per database | see `operations.md` [PG routine-vacuuming] |
| replication | `pg_stat_replication` lag; `pg_replication_slots` retained WAL | an unconsumed slot fills the disk [PG runtime-config-replication] |
| locks | `log_lock_waits` lines; `pg_locks` joined to `pg_stat_activity` | find the blocker after the fact [PG runtime-config-logging] |
| slow plans | `auto_explain` with `log_min_duration` ≥ 3 s, `log_timing = off`, `sample_rate` on busy systems | `log_analyze` with timing on costs every statement [PG auto-explain] |
| unused and invalid indexes | `pg_stat_user_indexes`, `pg_index.indisvalid` | see `indexing.md` |
| integer key headroom | `pg_sequences.last_value` | see `operations.md` |

`stats_fetch_consistency = 'snapshot'` in a diagnostic session so successive queries see one
generation of statistics [PG monitoring-stats].

## Exporting

- postgres_exporter or the provider's equivalent, connecting as `pg_monitor` (never superuser),
  credentials from a file (`DATA_SOURCE_PASS_FILE`) [postgres_exporter README].
- Application traces follow the OpenTelemetry database conventions: `db.system.name`,
  `db.namespace`, `db.operation.name`, `db.collection.name`, `db.query.summary`; span name
  `{db.query.summary}`; `db.query.text` sanitised; `db.query.parameter.*` opt-in only [OTel db
  semconv].
- Logs as `jsonlog` `[pg>=15]` or a `log_line_prefix` that carries user, database and
  application name (`operations.md`).

## Health probes

- **Readiness** checks the database through the real pool with a trivial timed query and, where
  the application has a schema guard, that the schema is at the expected version. A failing
  readiness removes the instance from the load balancer, which is what you want when the
  database is unreachable.
- **Liveness** never touches the database. Kubernetes' own guidance: liveness must "truly
  indicate unrecoverable application failure, for example a deadlock"; a liveness probe that
  depends on the database turns a database blip into a restart of every healthy instance, a
  cascading failure [Kubernetes probes].
- The container's `pg_isready` healthcheck says the server accepts connections; it is not the
  application's readiness [official image docs].

## Sources

- https://www.postgresql.org/docs/current/backup-dump.html
- https://pgbackrest.org/user-guide.html
- https://www.postgresql.org/docs/current/pgupgrade.html
- https://www.postgresql.org/docs/current/pgstatstatements.html
- https://www.postgresql.org/docs/current/monitoring-stats.html
- https://www.postgresql.org/docs/current/routine-vacuuming.html
- https://www.postgresql.org/docs/current/runtime-config-replication.html
- https://www.postgresql.org/docs/current/runtime-config-logging.html
- https://www.postgresql.org/docs/current/auto-explain.html
- https://github.com/prometheus-community/postgres_exporter
- https://opentelemetry.io/docs/specs/semconv/database/database-spans/
- https://kubernetes.io/docs/concepts/workloads/pods/probes/
- https://github.com/docker-library/docs/blob/master/postgres/README.md
