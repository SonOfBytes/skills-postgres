# Connection Pooling

Every Postgres connection is a process with its own memory; a few hundred is a lot. Pool on the
application side always; add a server-side pooler when many instances or many short-lived
processes share one database. Driver-specific fields are in `drivers/`.

## Size

- Start from the wiki's formula, `(cores × 2) + effective spindles` (spindles ≈ 0 when the
  working set is cached), and load-test upward. Past saturation more connections mean more
  context switches, `work_mem` pressure and lock contention, not more throughput. The number is
  deliberate, written down, and smaller than most people expect [wiki connections].
- `max_connections` on the server is strictly greater than the sum of every pool that can reach
  it, leaving slots for maintenance and monitoring [wiki connections].
- Each application instance's maximum times the instance count is the number that must fit;
  defaults (pgx: `max(4, NumCPU)`; Npgsql: 100 per connection string per process) are unrelated
  to the database's capacity [pgxpool docs; Npgsql connection strings].

## Application-side pool

Set explicitly, whatever the driver:

| Setting | Why |
|---|---|
| maximum connections | tied to database capacity across all instances |
| minimum or idle connections | so the first request after a quiet period does not pay a connect |
| maximum lifetime **with jitter** | replicas started together otherwise reconnect together |
| idle timeout | so a pooler or the server can reclaim |
| health-check period | so a dead connection is dropped before a request finds it |
| `application_name`, `connect_timeout` | attribution in `pg_stat_activity`; fail fast on a dead host |

Per-connection setup (`statement_timeout`, `search_path`, `TimeZone`) goes in the connection
string `options` or in role settings, not in a post-connect `SET`, because a server-side pooler's
`DISCARD ALL` erases it [PgBouncer config; Npgsql connection strings].

## Server-side pooler

- **PgBouncer** by default: single binary, transaction pooling, `~2 MB` per thousand clients.
  **PgCat** when replica routing or sharding is a documented need [PgBouncer features; PgCat
  README].
- `pool_mode = transaction` for web backends. It breaks, and the application must not use:
  session `SET`/`RESET` (use `SET LOCAL`), `LISTEN`/`NOTIFY`, `WITH HOLD` cursors, named
  `PREPARE`/`DEALLOCATE`, `LOAD`, session-level advisory locks, `PRESERVE ROWS`/`DELETE ROWS`
  temp tables [PgBouncer features].
- Protocol-level prepared statements work in transaction mode only with `max_prepared_statements
  > 0` (PgBouncer 1.21+). With it, pgx `cache_statement` and Npgsql auto-prepare are fine;
  without it, pgx uses `default_query_exec_mode=exec` and Npgsql `Max Auto Prepare=0` [PgBouncer
  config; pgx docs].
- TLS both sides: `client_tls_sslmode` defaults to `disable`, `server_tls_sslmode` to `prefer`
  (unauthenticated); set both. `auth_type = scram-sha-256` with `auth_query` rather than a
  committed `userlist.txt`. `application_name_add_host = 1` for attribution [PgBouncer config].
- `default_pool_size × databases × users + reserve_pool_size < max_connections − margin`;
  `query_wait_timeout` and `server_connect_timeout` bounded so saturation fails fast [PgBouncer
  config].

## Sizing example

Three application instances, `MaxConns` 10 each, one migrator, one exporter, two admins:
pool total 30, margin 10, `max_connections = 40` is enough and `100` is not "safer". If the
database has four cores and the working set is cached, ten per instance is already generous;
measure.

## Sources

- https://wiki.postgresql.org/wiki/Number_Of_Database_Connections
- https://pkg.go.dev/github.com/jackc/pgx/v5/pgxpool
- https://www.npgsql.org/doc/connection-string-parameters.html
- https://www.pgbouncer.org/config.html
- https://www.pgbouncer.org/features.html
- https://github.com/postgresml/pgcat
- https://pkg.go.dev/github.com/jackc/pgx/v5
