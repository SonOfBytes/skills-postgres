# Testing the Data Layer

The database is where the invariants live, so the tests that matter run against a real one. A
test that has never been seen to fail proves nothing; for every constraint or isolation test,
break the thing once and watch it go red.

## The fixture

- A real Postgres container (compose, or testcontainers' postgres module), never SQLite, an
  in-memory provider or a mocked driver: types, constraints, isolation and dialect are exactly
  what a substitute does not reproduce [testcontainers Go guide].
- One container per suite; isolation by resetting state, not by recreating the container
  [testcontainers Go guide].
- The schema comes from the **real migrations**, applied once; then snapshot and restore
  between tests (testcontainers `Snapshot`/`Restore`), or truncate every table. A hand-written
  test schema diverges from production by construction [testcontainers postgres module].
- The test database is separate from the development database; a fixture that truncates the
  database the developer is running against is the bug you find at the worst time.
- The test image's major matches production's; the store tests otherwise prove behaviour on the
  wrong version.
- Do **not** wrap each test in a transaction that is rolled back when the code under test
  manages its own transactions: production code is forced into subtransactions, and
  commit-visible behaviour (`NOTIFY`, deferred constraints, `SKIP LOCKED` across connections)
  becomes untestable [testcontainers postgres module; PG sql-notify].
- The integration tests must not silently skip when the database URL is unset in CI or the
  local gate; a gate that runs zero store tests reports success while proving nothing.

## What to test, per store method

| Path | Test |
|---|---|
| happy | seed, call, assert the rows returned or persisted (not the SQL text) |
| not found | the project's not-found convention, exactly |
| each constraint the method can trip | the violating row, asserting SQLSTATE 23505 / 23503 / 23514 through the driver's error type |
| tenant isolation | tenant A's call returns none of tenant B's rows, connected as the **application role** (an owner bypasses RLS unless forced) [PG ddl-rowsecurity] |
| ordering rules | ties on the ordering column (two rows with the same timestamp), the cursor at the last row, an empty set |
| concurrency | two real connections: the second `SKIP LOCKED` claim skips the first's row; a serialization failure surfaces as 40001 [PG explicit-locking; transaction-iso] |
| index use | `EXPLAIN` asserting an Index Scan on the expected index, only after seeding past the seq-scan crossover (state the count) [pganalyze planning basics] |

## Migrations

- CI applies every migration from an empty database (the fixture does this for free).
- If down files are policy, CI runs `up`, `down 1`, `up` on the newest pair.
- Every DDL in a migration session is preceded by `SET lock_timeout`; the runner retries. A
  migration that fails with a lock timeout in a test is a working safety net [postgres.ai].
- The schema guard's classifier (dirty / behind / ahead / fresh) is a pure function with a table
  test and no database.

## Database-resident logic

Constraints, triggers, functions and policies that the store tests do not exercise are tested with
pgTAP and `pg_prove` in CI; "pgTAP tests the part of your system that application tests skip"
[pgTAP].

```sql
BEGIN;
SELECT plan(2);
SELECT has_index('videos', 'videos_series_idx', 'series index exists');
SELECT col_not_null('videos', 'published_at', 'published_at is NOT NULL');
SELECT finish();
ROLLBACK;
```

## Verify the verifier

Before trusting a data-layer test, make it fail on purpose: drop the unique index and confirm the
23505 test fails; remove the tenant predicate and confirm the leak test fails; run the isolation
test as the owner and confirm it passes when it should not, then fix the connection role. A
green test that has never been red is an untested assertion, and the fixture mistakes that
produce false greens (connecting as owner, a ten-row table that seq-scans) are silent.

## Sources

- https://testcontainers.com/guides/getting-started-with-testcontainers-for-go/
- https://golang.testcontainers.org/modules/postgres/
- https://www.postgresql.org/docs/current/sql-notify.html
- https://www.postgresql.org/docs/current/ddl-rowsecurity.html
- https://www.postgresql.org/docs/current/explicit-locking.html
- https://www.postgresql.org/docs/current/transaction-iso.html
- https://pganalyze.com/docs/explain/basics-of-postgres-query-planning
- https://postgres.ai/blog/20210923-zero-downtime-postgres-schema-migrations-lock-timeout-and-retries
- https://pgtap.org/
