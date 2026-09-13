# Npgsql and EF Core (.NET)

Npgsql is the driver; EF Core through `Npgsql.EntityFrameworkCore.PostgreSQL` is the ORM most
.NET projects use on top of it. The rules below are the ones that differ from generic .NET
advice because Postgres is underneath.

## Connection string and pool

Defaults [Npgsql connection strings]: `Maximum Pool Size` 100 **per connection string per
process** (multiply by instances and compare with `max_connections`), `Connection Idle
Lifetime` 300 s, `Connection Pruning Interval` 10 s, `Connection Lifetime` 3600 s (no jitter),
`Command Timeout` 30 s, `Timeout` (connect) 15 s.

```
Host=db;Database=app;Username=app;Password=…;SSL Mode=VerifyFull;Root Certificate=/etc/ssl/ca.pem;
Maximum Pool Size=20;Minimum Pool Size=2;Connection Lifetime=3600;Command Timeout=15;
Application Name=app-web;Options=-c statement_timeout=15s -c TimeZone=UTC
```

- `Options=-c …` applies per-connection settings at connect, surviving a pooler's reset; a
  post-connect `SET` does not [Npgsql connection strings].
- Behind PgBouncer transaction mode: `No Reset On Close=true` and `Max Auto Prepare=0` unless
  the pooler supports prepared statements [Npgsql connection strings; PgBouncer].
- `Enlist=false` where ambient `TransactionScope` is unwanted [Npgsql performance].

## Types

- `decimal` for `numeric`; `DateTime` with `Kind = Utc` for `timestamptz` (Npgsql 6+ rejects
  `Unspecified`), `DateTimeOffset` where the offset matters; `Guid` for `uuid`; `string` for
  `text`; `JsonDocument` or POCO mapping for `jsonb`.

## Performance

- Prepare hot commands (`Prepare()`) or enable `Max Auto Prepare` (not behind transaction
  pooling); batch with `NpgsqlBatch`; bulk-insert with `BeginBinaryImport` (COPY); async APIs
  throughout [Npgsql performance].

```csharp
await using var batch = new NpgsqlBatch(conn)
{
    BatchCommands =
    {
        new("UPDATE channels SET last_polled_at = now() WHERE id = $1") { Parameters = { new() { Value = id } } },
        new("INSERT INTO poll_log (channel_id) VALUES ($1)") { Parameters = { new() { Value = id } } },
    }
};
await batch.ExecuteNonQueryAsync(ct);
```

## EF Core

- Lazy loading off: it "makes it extremely easy to inadvertently trigger the N+1 problem"; eager
  `Include` or explicit loading makes every round trip visible [EF Core efficient querying].
- Read paths: `AsNoTracking()` and projections (`Select` into a DTO); identity resolution is
  lost, which is fine for reads [EF Core].
- Multiple collection `Include`s: `AsSplitQuery()` against cartesian explosion, accepting that
  the split queries are not one snapshot [EF Core].
- Keyset pagination, not `Skip`/`Take`, on lists that grow [EF Core].
- Raw SQL only through `FromSql($"…{value}…")` / `ExecuteSql($"…")`, which parameterise the
  interpolation; never `FromSqlRaw` with concatenation [EF Core].
- Constraints in the model so the database enforces them: `HasIndex().IsUnique()`,
  `HasCheckConstraint`, and an explicit `OnDelete(DeleteBehavior.Cascade | SetNull | Restrict)`
  on every relationship.
- Every query bounded (`Take`) or streamed (`AsAsyncEnumerable`).

```csharp
var page = await db.Videos.AsNoTracking()
    .Where(v => v.ChannelId == channelId
             && (v.PublishedAt > after || (v.PublishedAt == after && v.Id.CompareTo(afterId) > 0)))
    .OrderBy(v => v.PublishedAt).ThenBy(v => v.Id)
    .Take(50)
    .Select(v => new VideoRow(v.Id, v.Title, v.PublishedAt))
    .ToListAsync(ct);
```

## Migrations

- Generated migrations are reviewed as the SQL they produce (`dotnet ef migrations script
  --idempotent`): EF emits non-concurrent `CREATE INDEX`, blocking type changes and `NOT NULL`
  with a full scan. Replace those with `migrationBuilder.Sql(…, suppressTransaction: true)`
  carrying `CONCURRENTLY` or `NOT VALID` (`migration-recipes.md`) [EF Core migrations].
- `Database.Migrate()` at start races across replicas; run `dotnet ef database update` or a
  migration bundle as a release step, and validate at readiness by reading
  `__EFMigrationsHistory` (`migrations.md`).

## Transactions and errors

- `IsolationLevel.Serializable` only with a retry on `PostgresException.SqlState == "40001"`
  around the whole unit; `NpgsqlRetryingExecutionStrategy` retries transient failures but does
  not make an unkeyed insert safe (`transactions.md`).
- `SET LOCAL` through `ExecuteSqlRaw("SET LOCAL …")` inside the transaction.
- Map `PostgresException.SqlState` at the repository boundary (`23505`, `23503`, `23514`,
  `40001`, `40P01`, `57014`); unwrap `DbUpdateException.InnerException`. Message text never
  reaches a response.

```csharp
catch (DbUpdateException e) when (e.InnerException is PostgresException { SqlState: PostgresErrorCodes.UniqueViolation })
{
    throw new AlreadyExistsException();
}
```

## Testing

Testcontainers for .NET (`new PostgreSqlBuilder().WithImage("postgres:16").Build()`), never the
InMemory or SQLite provider, whose constraint and type semantics differ. Assert
`PostgresException.SqlState` on failure paths [Testcontainers .NET].

## Sources

- https://www.npgsql.org/doc/connection-string-parameters.html
- https://www.npgsql.org/doc/performance.html
- https://www.pgbouncer.org/features.html
- https://learn.microsoft.com/en-us/ef/core/performance/efficient-querying
- https://learn.microsoft.com/en-us/ef/core/managing-schemas/migrations/
- https://dotnet.testcontainers.org/modules/postgres/
