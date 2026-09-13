# sqlc

sqlc compiles SQL query files into type-safe code. The query files are the source of truth and
the review subject; the generated code is a build artefact.

## Configuration

```yaml
version: "2"
sql:
  - engine: "postgresql"
    schema: "internal/store/migrations"
    queries: "internal/store/queries"
    gen:
      go:
        package: "sqlcgen"
        out: "internal/store/sqlcgen"
        sql_package: "pgx/v5"
        emit_pointers_for_null_types: true
    strict_function_checks: true
    strict_order_by: true
```

- `schema` points at the **real migrations** so a hand-maintained schema cannot drift; sqlc
  reads golang-migrate and goose files, ignoring down sections [sqlc config].
- `sql_package: "pgx/v5"` keeps binary parameters and `CopyFrom`; `database/sql` loses both.
- `emit_pointers_for_null_types` or `emit_result_struct_pointers` chosen once and documented.
- `sqlc generate` runs in CI with a diff check, so a query that no longer compiles against the
  schema fails the build; `sqlc vet` with `sqlc/db-prepare` proves each query prepares against a
  live database [sqlc vet].

## Query files

- Every query is annotated: `-- name: NextEpisode :one`, `:many`, `:exec`, `:execrows`,
  `:batchexec`, `:copyfrom`. A `:many` on a growing table carries a `LIMIT`.
- Lists as one array parameter: `WHERE id = ANY($1::uuid[])` generates a slice argument;
  `sqlc.slice()` only for engines without arrays. The slice must be non-nil and non-empty at the
  call site [sqlc select howto].
- `sqlc.arg(name)` for readable parameters and `sqlc.narg(name)` for nullable ones.
- Explicit column lists: sqlc expands `SELECT *` at generate time, so a schema change silently
  changes the generated struct.

```sql
-- name: NextEpisode :one
SELECT v.id, v.youtube_video_id, v.title, v.published_at
FROM videos v
JOIN channel_approvals ca
  ON ca.channel_id = v.channel_id
 AND ca.household_id = sqlc.arg(household_id)
 AND ca.status = 'active'
WHERE v.channel_id = sqlc.arg(channel_id)
  AND (sqlc.narg(after_published_at)::timestamptz IS NULL
       OR (v.published_at, v.id) > (sqlc.narg(after_published_at), sqlc.narg(after_id)))
ORDER BY v.published_at ASC, v.id ASC
LIMIT 1;
```

## Using the generated code

- The generated `Queries` type is wrapped by the project's store; handlers never call it.
- Transactions: `q.WithTx(tx)` inside `pool.Begin`; `SET LOCAL` through `tx.Exec` before the
  queries (`drivers/pgx.md`).
- Errors are mapped in the wrapper on `*pgconn.PgError`; generated code is never edited.

## Testing

The wrapper's store tests run against a real Postgres (`testing.md`); the generated code has no
tests of its own. `sqlc generate` plus `git diff --exit-code` in CI proves the queries and the
schema still agree.

## Sources

- https://docs.sqlc.dev/en/stable/reference/config.html
- https://docs.sqlc.dev/en/stable/howto/vet.html
- https://docs.sqlc.dev/en/stable/howto/select.html
