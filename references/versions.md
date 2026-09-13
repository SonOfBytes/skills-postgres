# Versions: What Each Major Adds, and When It Dies

Read this when a `[pg>=N]` or `[pg<=N]` marker is in doubt, or when choosing a version for a new
deployment. Support policy: five years per major, minor releases at least quarterly, and "always
run the current minor" [PG versioning]. Checked against postgresql.org on 13 September 2026.

## Majors

| Major | First release | End of life | Status |
|---|---|---|---|
| 14 | 30 Sep 2021 | 12 Nov 2026 | supported, upgrade this year |
| 15 | 13 Oct 2022 | 11 Nov 2027 | supported |
| 16 | 14 Sep 2023 | 9 Nov 2028 | supported |
| 17 | 26 Sep 2024 | 8 Nov 2029 | supported |
| 18 | 25 Sep 2025 | 14 Nov 2030 | supported, current |
| 19 | beta 3, 13 Aug 2026 | | beta, not for production |

Anything at 13 or below is unsupported: no security fixes. For a new deployment choose the newest
major that every managed provider or extension you need supports; for an upgrade, prefer
`pg_upgrade --check` then `--link` (irreversible once the new cluster starts) or logical
replication for near-zero downtime [PG pgupgrade].

## What changed, by feature area

Markers below are the ones the other references use.

### Types and schema

Majors below 14 are listed for the history only; no marker refers to them.

- **10** (unsupported): identity columns (`GENERATED ALWAYS AS IDENTITY`) replace `serial`.
- **11** (unsupported): adding a column with a non-volatile default no longer rewrites the table.
- **12** (unsupported): stored generated columns; `REINDEX CONCURRENTLY`; CTEs inline by default with
  `MATERIALIZED` / `NOT MATERIALIZED` to override.
- **13** (unsupported): `autovacuum_vacuum_insert_scale_factor` for insert-only tables; B-tree
  deduplication.
- `[pg>=14]` `DETACH PARTITION CONCURRENTLY`; BRIN `minmax_multi` and bloom opclasses;
  `max_slot_wal_keep_size`; `jsonb` subscripting; `CREATE INDEX` on partitioned tables in
  parallel; the official image defaults to `scram-sha-256`.
- `[pg>=15]` `MERGE`; `UNIQUE NULLS NOT DISTINCT`; `security_invoker` views; `jsonlog`
  destination; ICU per-database collation; `CREATE` on `public` revoked from `PUBLIC` for new
  databases and `public` owned by `pg_database_owner`.
- `[pg>=16]` SQL/JSON constructors and `IS JSON`; `pg_stat_io`; logical replication from a
  standby; `COPY` default-value mapping.
- `[pg>=17]` `MERGE … RETURNING` with `merge_action()`; `JSON_TABLE()`; B-tree handles `IN
  (…)` lists efficiently; `COPY … ON_ERROR ignore` and `LOG_VERBOSITY`; `transaction_timeout`;
  identity columns on partitioned tables; parallel BRIN build; builtin collation provider
  (`C.UTF-8`).
- `[pg>=18]` `uuidv7()` and `uuidv4()`; virtual generated columns as the default; named
  `NOT NULL` constraints and `NOT NULL … NOT VALID`; temporal `PRIMARY KEY … WITHOUT
  OVERLAPS` and `FOREIGN KEY … PERIOD`; `RETURNING OLD.x, NEW.x`; B-tree skip scan; `EXPLAIN
  ANALYZE` includes `BUFFERS`; `VACUUM`/`ANALYZE … ONLY` on partitioned parents; asynchronous
  I/O (`io_method`); data checksums on by default; `md5` authentication deprecated; OAuth
  authentication; `pg_upgrade` carries planner statistics; `COPY … REJECT_LIMIT`; the official
  image's `PGDATA` becomes version-specific.
- `[pg>=19]` (beta) JIT off by default; TOAST default compression `lz4`; `REPACK
  (CONCURRENTLY)`; `MERGE PARTITIONS` / `SPLIT PARTITION`; `COPY … ON_ERROR set_null`, `HEADER
  2`, JSON output.

### Behaviour that changed under you

- `[pg<=14]` `CREATE` on the `public` schema is granted to every role; revoke it explicitly
  (see `security.md`). Databases upgraded from 14 keep the old grant even on 15+.
- `[pg<=17]` `PGDATA` in the official image is `/var/lib/postgresql/data`; from 18 it is
  `/var/lib/postgresql/18/docker`. A volume mounted at the old path on the new image starts an
  empty cluster (see `operations.md`).
- `[pg<=17]` data checksums are off unless `initdb --data-checksums` was used.
- `[pg<=17]` generated columns default to `STORED`; from 18 the default is `VIRTUAL`, so a
  migration written without the keyword behaves differently across majors: always write the
  keyword.
- `[pg<=18]` JIT is on by default and hurts short OLTP queries; set `jit = off` per database
  (see `operations.md`).
- `[pg<=16]` no builtin collation provider; use ICU or `C.UTF-8` from the OS and reindex text
  indexes whenever glibc changes (see `operations.md`).
- `[pg<=17]` `pg_upgrade` drops planner statistics; run `vacuumdb --all --analyze-in-stages`
  immediately after or the first hours run on guesses.

## Client-side versions that matter

| Component | Note |
|---|---|
| PgBouncer 1.21+ | protocol-level prepared statements in transaction mode with `max_prepared_statements > 0` |
| pgx v5 | `QueryExecMode` selection; v4 is unmaintained |
| Npgsql 6+ | `DateTime` must carry `Kind`; `timestamptz` maps to UTC |
| libpg_query 18 (pglast 8.x) | what `verify/check_sqlblocks.py` parses with; 19-only syntax will not parse until libpg_query 19 |

## Sources

- https://www.postgresql.org/support/versioning/
- https://www.postgresql.org/docs/current/pgupgrade.html
- https://www.postgresql.org/docs/release/15.0/
- https://www.postgresql.org/docs/release/16.0/
- https://www.postgresql.org/docs/release/17.0/
- https://www.postgresql.org/docs/release/18.0/
- https://www.postgresql.org/about/news/postgresql-18-released-3142/
- https://www.crunchydata.com/blog/postgres-19-how-our-advice-has-changed-since-we-wrote-it
