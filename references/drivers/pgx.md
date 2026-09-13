# pgx (Go)

pgx v5 is the driver and toolkit; `database/sql` through `pgx/v5/stdlib` loses binary parameters
and `CopyFrom`. The seam module in `verify/pgx-seam/` compiles and tests everything below against
Postgres 16, 17 and 18.

## Pool

`*pgx.Conn` is not concurrency safe; every concurrent application uses `pgxpool.Pool` [pgx wiki].
Defaults are not sized for you: `MaxConns` is `max(4, NumCPU)`, `MinConns` 0, `MaxConnLifetime`
1h, `MaxConnIdleTime` 30m, `HealthCheckPeriod` 1m and `MaxConnLifetimeJitter` 0, so replicas
started together reconnect together [pgxpool docs]. Set them, and set per-connection parameters in
`RuntimeParams`, not with a `SET` after checkout.

```go
cfg, err := pgxpool.ParseConfig(dsn)
if err != nil {
	return nil, fmt.Errorf("parse database url: %w", err)
}
cfg.MaxConns = 10
cfg.MinConns = 2
cfg.MaxConnLifetime = time.Hour
cfg.MaxConnLifetimeJitter = 10 * time.Minute
cfg.MaxConnIdleTime = 15 * time.Minute
cfg.HealthCheckPeriod = time.Minute
cfg.ConnConfig.ConnectTimeout = 5 * time.Second
cfg.ConnConfig.RuntimeParams["application_name"] = "kvs-web"
cfg.ConnConfig.RuntimeParams["TimeZone"] = "UTC"
pool, err := pgxpool.NewWithConfig(ctx, cfg)
```

The same through the URL: `?pool_max_conns=10&pool_max_conn_lifetime=1h&pool_max_conn_lifetime_jitter=10m&application_name=kvs-web`.

## Execution mode

`QueryExecMode` must match the deployment [pgx docs]:

| Deployment | Mode | Why |
|---|---|---|
| direct, or PgBouncer session mode | `cache_statement` (default) | prepared statements cached per connection |
| PgBouncer transaction mode with `max_prepared_statements > 0` (1.21+) | `cache_statement` | the pooler rewrites and reuses statements |
| PgBouncer transaction mode otherwise | `exec` | no server-side statement; binary parameters kept |
| last resort | `simple_protocol` | text parameters; loses binary types |
| never behind transaction pooling | `describe_exec` | describe and execute may land on different server connections |

`cfg.ConnConfig.DefaultQueryExecMode = pgx.QueryExecModeExec` or
`?default_query_exec_mode=exec`.

## Queries

- `pgx.CollectRows` with `pgx.RowToStructByName` (or `RowToStructByNameLax`) for new code rather
  than positional `rows.Scan` lists, which break silently when a column is added or reordered
  [pgx docs]. In a store whose methods all scan positionally with an explicit column list beside
  the scan, match the surrounding style; consistency in one package beats a mixed idiom.
- `pgx.NamedArgs` when a query has many parameters; typed parameters (`uuid.UUID`, `[]int64`
  for `= ANY($1::bigint[])`, `time.Time` for `timestamptz`).
- `numeric` scans into `pgtype.Numeric` or a decimal type, never `float64`; `timestamptz` scans
  to `time.Time` in the session zone, hence `TimeZone=UTC` above [pgx docs].
- `SendBatch` for several independent statements in one round trip; `CopyFrom` for bulk inserts
  [pgx docs].

```go
rows, err := pool.Query(ctx, `SELECT id, title, published_at FROM videos WHERE id = ANY($1)`, ids)
if err != nil {
	return nil, fmt.Errorf("select videos: %w", err)
}
videos, err := pgx.CollectRows(rows, pgx.RowToStructByName[Video])
```

## Transactions

```go
tx, err := pool.Begin(ctx)
if err != nil {
	return fmt.Errorf("begin: %w", err)
}
defer tx.Rollback(ctx) // no-op after Commit

if _, err := tx.Exec(ctx, `SET LOCAL statement_timeout = '5s'`); err != nil {
	return fmt.Errorf("set timeout: %w", err)
}
// work
if err := tx.Commit(ctx); err != nil {
	return fmt.Errorf("commit: %w", err)
}
```

`SET LOCAL` through `tx.Exec`; never `pool.Exec(ctx, "SET …")`, which lands on an arbitrary
connection and leaks. `pool.BeginTx` with `pgx.TxOptions{IsoLevel: pgx.Serializable}` when the
unit needs it, with the 40001 retry around the whole function (`transactions.md`).

## Errors

```go
var pgErr *pgconn.PgError
if errors.As(err, &pgErr) {
	switch pgErr.Code {
	case pgerrcode.UniqueViolation:
		return ErrAlreadyExists
	case pgerrcode.ForeignKeyViolation:
		return ErrInvalidReference
	case pgerrcode.CheckViolation:
		return ErrInvalidInput
	case pgerrcode.SerializationFailure, pgerrcode.DeadlockDetected:
		return errRetryable
	}
}
if errors.Is(err, pgx.ErrNoRows) {
	return ErrNotFound
}
```

The SQLSTATE constants come from the separate module `github.com/jackc/pgerrcode`.
`pgErr.ConstraintName` is a fine secondary key when the project names constraints deliberately.
`pgErr.Message` never reaches a client. `context.DeadlineExceeded` from the caller's context is
not retryable.

## Tracing

`ConnConfig.Tracer` sees `TraceQueryStartData.Args`. Log them only at a level the configuration
keeps off in production; log the SQL and the command tag otherwise (`security.md`).

## Testing

testcontainers-go's postgres module: `postgres.Run(ctx, "postgres:16",
postgres.WithInitScripts(...), postgres.BasicWaitStrategies())`, then `Snapshot` after the
migrations and `Restore` between tests. Two connections for concurrency tests: `pool.Acquire`
twice. Failure paths assert `pgErr.Code` [testcontainers postgres module].

## Sources

- https://github.com/jackc/pgx/wiki/Getting-started-with-pgx
- https://pkg.go.dev/github.com/jackc/pgx/v5/pgxpool
- https://pkg.go.dev/github.com/jackc/pgx/v5
- https://pkg.go.dev/github.com/jackc/pgx/v5/pgconn
- https://golang.testcontainers.org/modules/postgres/
