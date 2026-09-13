# Operations: Settings, Vacuum, the Container, Collation

What to set on the server and the container so the database stays healthy without attention.
Backups, monitoring and probes are in `backup-and-observability.md`. On a managed service most
of the server section is the provider's parameter group; set what it exposes.

## Timeouts, per role

Never in `postgresql.conf`, where they would also kill maintenance [PG runtime-config-client]:

```sql
ALTER ROLE app SET statement_timeout = '15s';
ALTER ROLE app SET lock_timeout = '2s';
ALTER ROLE app SET idle_in_transaction_session_timeout = '30s';
ALTER ROLE migrator SET statement_timeout = '1h';
ALTER ROLE migrator SET lock_timeout = '5s';
```

- `lock_timeout` below `statement_timeout`, or it is pointless [PG runtime-config-client].
- `idle_in_transaction_session_timeout` everywhere: an idle transaction holds locks and the xmin
  horizon [PG runtime-config-client].
- `[pg>=17]` `transaction_timeout` as the outer bound on any transaction, explicit or implicit
  [PG runtime-config-client].
- `idle_session_timeout` only with care behind a pooler, which may not expect a server-side close
  [PG runtime-config-client].

## Server settings that every application database wants

| Setting | Value | Why |
|---|---|---|
| `shared_buffers` | about a quarter of RAM (15 to 25 per cent, measure) | more double-buffers against the OS cache [EDB memory] |
| `effective_cache_size` | about half of RAM | planner hint only; safe to set high [EDB memory] |
| `work_mem` | RAM × 0.25 ÷ `max_connections`, per sort or hash node | this is why `max_connections` is not free [EDB memory; wiki connections] |
| `maintenance_work_mem` | ~5 per cent of RAM, more during index builds | index builds and vacuum [EDB memory] |
| `max_connections` | just above the pool total | see `pooling.md` |
| `jit` | `off` per database `[pg<=18]` | JIT costs more than it saves on short OLTP queries; off by default from 19 [PG jit-decision; Crunchy 19] |
| `shared_preload_libraries` | `pg_stat_statements` | needs a restart; `compute_query_id = on`, `track_planning = off` [PG pgstatstatements] |
| `track_io_timing` | `on` | without it I/O time is invisible [PG monitoring-stats] |
| `log_lock_waits` | `on` | logs waits beyond `deadlock_timeout` [PG runtime-config-logging] |
| `log_min_duration_statement` | 250 ms to 1 s, with `log_min_duration_sample` | slow-query visibility without logging everything [PG runtime-config-logging] |
| `log_checkpoints`, `log_autovacuum_min_duration` | on; below ten minutes on busy tables | cheapest I/O and vacuum signals [PG runtime-config-logging] |
| `log_line_prefix` | `'%m [%p] %q%u@%d/%a '` or `log_destination = 'jsonlog'` `[pg>=15]` | attribute a line to a service [PG runtime-config-logging] |
| `wal_level` | `replica` (or `logical`) | `minimal` forbids replication and slots [PG runtime-config-replication] |
| `max_slot_wal_keep_size` | bounded, wherever a slot exists | an abandoned slot otherwise fills the disk [PG runtime-config-replication] |
| `synchronous_commit` | chosen deliberately; `off` per transaction for low-value writes | latency against durability [PG runtime-config-replication] |
| `random_page_cost` | 1.1 on SSD | the 4.0 default assumes spinning disks [PG runtime-config-query] |

## Autovacuum

- Never off. Watch `age(datfrozenxid)` per database and `age(relfrozenxid)` per table: warnings
  begin 40 million transactions before wraparound, writes stop 3 million before [PG
  routine-vacuuming].
- Four things hold back the horizon: long transactions, prepared transactions, long-lived
  backends (`age(backend_xmin)`), stale replication slots [PG routine-vacuuming].
- Hot tables get per-table settings; the 20 per cent default means a 100-million-row table
  collects 20 million dead tuples before vacuum triggers [Cybertec autovacuum].

```sql
ALTER TABLE package_jobs SET (autovacuum_vacuum_scale_factor = 0.01);
ALTER TABLE watch_events SET (autovacuum_vacuum_insert_scale_factor = 0.05);
ALTER TABLE videos SET (autovacuum_analyze_scale_factor = 0, autovacuum_analyze_threshold = 100000);
```

- On SSD raise `autovacuum_vacuum_cost_limit` or lower `autovacuum_vacuum_cost_delay`; the
  defaults throttle for spinning disks. Raise `autovacuum_max_workers` on partitioned schemas
  [Cybertec autovacuum].
- `ANALYZE` after bulk loads and on partitioned parents [PG routine-vacuuming].
- Bloat: `pgstattuple_approx()` (grant `pg_stat_scan_tables`), fix with `REINDEX … CONCURRENTLY`
  or `[pg>=19]` `REPACK (CONCURRENTLY)`; never `VACUUM FULL` as a wraparound remedy [PG
  pgstattuple; sql-reindex; PG routine-vacuuming].

## Integer key headroom

For every `int` sequence, compare `pg_sequences.last_value` with 2,147,483,647 on a schedule;
the migration to `bigint` (`migration-recipes.md`) takes longer than the runway you have left
when you notice [Crunchy serials].

## Collation

- Choose deliberately: `[pg>=17]` the builtin provider (`C.UTF-8`) is immutable across OS
  upgrades; otherwise ICU. A glibc locale is tied to the glibc version, and glibc 2.28 silently
  broke text B-tree order; reindex every text index in the same window as any glibc or ICU
  change [pgEdge collation; thebuild.com collations]. Encoding `UTF8`; timezone `UTC`.

## The official container image

- The volume mounts at the exact `PGDATA`: `[pg<=17]` `/var/lib/postgresql/data`; `[pg>=18]`
  `/var/lib/postgresql/18/docker`. A parent mount, or an unchanged mount across the 18 tag bump,
  starts an empty cluster [official image docs].
- Pin a major (`postgres:16`), never `latest`; a major bump is a `pg_upgrade`.
- `POSTGRES_PASSWORD_FILE` from a secret, never a literal; never
  `POSTGRES_HOST_AUTH_METHOD=trust` [official image docs].
- `[pg<=17]` `POSTGRES_INITDB_ARGS="--data-checksums"` (18 defaults on); checksums are how you
  find silent storage corruption [official image docs; PG 18 release notes].
- Runs as `postgres`, never root; `shm_size` above the 64 MB default; a `healthcheck` with
  `pg_isready` gating dependent services, which says "accepting connections", not "schema ready"
  [official image docs].
- `/docker-entrypoint-initdb.d` runs only on an empty data directory: roles and databases at
  most; migrations own the schema [official image docs].
- The port is published to the host only in development; in production the service sits on an
  internal network [OWASP].
- Settings by `command: postgres -c shared_preload_libraries=pg_stat_statements -c …` or a
  mounted `postgresql.conf`.

## Version support

Supported majors, end-of-life dates and the upgrade path are in `versions.md`.

## Sources

- https://www.postgresql.org/docs/current/runtime-config-client.html
- https://www.enterprisedb.com/postgres-tutorials/how-tune-postgresql-memory
- https://wiki.postgresql.org/wiki/Number_Of_Database_Connections
- https://www.postgresql.org/docs/current/jit-decision.html
- https://www.crunchydata.com/blog/postgres-19-how-our-advice-has-changed-since-we-wrote-it
- https://www.postgresql.org/docs/current/pgstatstatements.html
- https://www.postgresql.org/docs/current/monitoring-stats.html
- https://www.postgresql.org/docs/current/runtime-config-logging.html
- https://www.postgresql.org/docs/current/runtime-config-replication.html
- https://www.postgresql.org/docs/current/runtime-config-query.html
- https://www.postgresql.org/docs/current/routine-vacuuming.html
- https://www.cybertec-postgresql.com/en/tuning-autovacuum-postgresql/
- https://www.postgresql.org/docs/current/pgstattuple.html
- https://www.postgresql.org/docs/current/sql-reindex.html
- https://www.crunchydata.com/blog/postgres-serials-should-be-bigint-and-how-to-migrate
- https://www.pgedge.com/blog/what-is-a-collation-and-why-is-my-data-corrupt
- https://thebuild.com/blog/2024/11/15/the-doom-that-came-to-postgresql-when-collations-change/
- https://github.com/docker-library/docs/blob/master/postgres/README.md
- https://www.postgresql.org/docs/release/18.0/
- https://cheatsheetseries.owasp.org/cheatsheets/Database_Security_Cheat_Sheet.html
