# Transactions, Isolation and Locking

What a transaction sees, what it holds, and what to do when two collide. Job queues have their
own file (`queues.md`); server-side timeouts are in `operations.md`.

## Isolation levels

- **Read Committed** (default) takes a new snapshot **per statement**. A read followed by a
  dependent write in the same transaction is a lost update unless the row is locked or the write
  re-checks a version [PG transaction-iso].
- **Repeatable Read** gives one snapshot for the whole transaction; a write to a row another
  transaction changed since the snapshot fails with `could not serialize access due to concurrent
  update`, and you retry [PG transaction-iso].
- **Serializable** detects every anomaly, at the price of retries you cannot predict: "it will be
  necessary to have a generalized way of handling serialization failures (SQLSTATE 40001)".
  Retry the **whole** transaction. Keep them short, declare read-only ones `READ ONLY`, avoid
  redundant `FOR UPDATE`, prefer index scans (a seq scan takes a relation-level predicate lock),
  and expect a unique violation even after checking the key is absent [PG transaction-iso].

## Read-modify-write

Three correct shapes; pick by the gap between read and write.

```sql
-- 1. Same transaction, milliseconds apart: lock the row.
BEGIN;
SELECT balance FROM accounts WHERE id = $1 FOR NO KEY UPDATE;
UPDATE accounts SET balance = balance - $2 WHERE id = $1;
COMMIT;

-- 2. Same statement: let the database do the arithmetic.
UPDATE accounts SET balance = balance - $2 WHERE id = $1 AND balance >= $2
RETURNING balance;

-- 3. Across a user round-trip: optimistic version; zero rows updated means a conflict.
UPDATE documents
SET body = $2, version = version + 1
WHERE id = $1 AND version = $3;
```

The shape that looks safe and is not: `UPDATE t SET taken = … WHERE id = (SELECT id FROM t WHERE
free … LIMIT 1)`. The uncorrelated subquery runs once; a second session that blocks on the row
lock re-checks only the outer `id = …` after the first commits, which still holds, so both callers
are told they won. Repeat the predicate in the outer `WHERE` and add `FOR UPDATE SKIP LOCKED` to
the subquery [PG transaction-iso; PG explicit-locking]:

```sql
UPDATE invites SET claimed_at = now()
WHERE claimed_at IS NULL
  AND id = (
    SELECT id FROM invites
    WHERE lower(email) = lower($1) AND claimed_at IS NULL
    ORDER BY created_at
    LIMIT 1
    FOR UPDATE SKIP LOCKED
  )
RETURNING id;
```

`FOR NO KEY UPDATE` when you update non-key columns: a plain `FOR UPDATE` blocks `FOR KEY
SHARE`, which is exactly the lock a child-row insert takes on the parent [PG explicit-locking].
`FOR UPDATE OF t` in a join locks only the table you intend to change.

## What may be retried

| SQLSTATE | Meaning | Retry? |
|---|---|---|
| 40001 | serialization_failure | yes, whole transaction, backoff with jitter |
| 40P01 | deadlock_detected | yes, whole transaction |
| 57P01 | admin_shutdown | yes, after reconnect |
| 55P03 | lock_not_available (`lock_timeout`, `NOWAIT`) | yes, bounded |
| 08xxx | connection exceptions | yes, after reconnect |
| 23505, 23503, 23514 | integrity violations | **never**; map to a domain error |
| 25P02 | in_failed_sql_transaction | never; roll back first |
| 57014 | query_canceled by your own deadline | never automatically |

Retry only an operation that is idempotent or carries an idempotency key; "retry on error"
around an unkeyed insert duplicates the insert [PG errcodes-appendix].

## Deadlocks

Acquire locks on multiple objects in a consistent order (by primary key, ascending, across
every code path), and take the most restrictive mode you will need first. Deadlock detection
still fires after `deadlock_timeout`; handle 40P01 [PG explicit-locking].

## Advisory locks

- `pg_advisory_xact_lock(key)` for application mutexes: released at commit or rollback, so a
  crash cannot leak it. Session-level `pg_advisory_lock` stacks (three locks need three unlocks),
  leaks across a pooled connection, and does not work through transaction pooling [PG
  functions-admin; PgBouncer features].
- Derive the key from a stable hash of the resource (`hashtext('channel:' || id)`) so two
  code paths agree.

```sql
BEGIN;
SELECT pg_advisory_xact_lock(hashtext('backfill:' || $1::text));
-- work that must not run twice for the same channel
COMMIT;
```

## Transaction boundaries

- One owner per invariant: the service or store method that must be atomic opens the
  transaction; handlers do not. Multi-write operations that must be all-or-nothing run in one
  transaction, so partial creation is impossible.
- Nothing external inside a transaction: no user interaction, no HTTP call, no message broker.
  Commit local state, call out, resume from a stored recovery point [Brandur idempotency keys].
  The staged-drain pattern for enqueueing is in `queues.md`.
- Short. A transaction held across a loop of external work holds locks and the xmin horizon,
  which blocks vacuum for the whole database (`operations.md`).
- After any error the transaction is `25P02`: only `ROLLBACK` or `ROLLBACK TO SAVEPOINT`
  proceeds. Savepoints are deliberate, not an error-swallowing device; each costs a
  subtransaction slot [PG errcodes-appendix].

## Per-transaction settings

`SET LOCAL`, never `SET`: a session-level `SET` outlives the transaction and lands on whoever
next checks out that pooled connection [PG sql-set]. Tenant context for row-level security is
the canonical case (`rls.md`); `statement_timeout`, `work_mem` and `plan_cache_mode` are the
others.

```sql
BEGIN;
SET LOCAL statement_timeout = '5s';
SET LOCAL app.tenant_id = '0f2c…';
-- queries
COMMIT;
```

The caller's context deadline or cancellation token is mirrored server-side by a per-role
`statement_timeout` or the `SET LOCAL` above; cancelling the client alone leaves the server
running until it notices.

## Idempotency keys

For any operation that is externally visible and not naturally idempotent (a charge, an outbound
message): a key supplied by the client, `UNIQUE (scope_id, key)`, a stored request fingerprint,
a `recovery_point`, and a `locked_at` to keep two retries from running at once [Brandur
idempotency keys].

```sql
CREATE TABLE idempotency_keys (
  scope_id        uuid NOT NULL,
  key             text NOT NULL,
  request_hash    bytea NOT NULL,
  recovery_point  text NOT NULL DEFAULT 'started',
  response_code   int,
  response_body   jsonb,
  locked_at       timestamptz,
  created_at      timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (scope_id, key)
);
```

Protocol: insert the key (or find it); if found with a different `request_hash`, reject; if
finished, return the stored response; otherwise take the lock (`locked_at IS NULL` in the
`UPDATE`), run the next atomic phase, advance `recovery_point`, commit, repeat.

## LISTEN / NOTIFY

A best-effort wake-up, not a queue: payload up to 8000 bytes, delivered only to listeners
connected at commit, identical notifications within one transaction collapsed, and a full
notification queue (8 GB) fails `NOTIFY` at commit. Send a key and re-query on wake. Does not
work through transaction pooling [PG sql-notify; PgBouncer features].

## MVCC

Every `UPDATE` writes a new row version; an update that touches an indexed column writes every
index. A counter incremented per request, a heartbeat on a wide row, or a 1 Hz progress write is
churn: batch it, debounce it, or move the hot column to its own narrow table [PG storage-hot].

## Replicas

A read routed to a replica is stale by the replication lag; a write followed by a read of the
same row on a replica is a read-your-writes bug. Route by consistency need, not by verb [PG
hot-standby].

## Sources

- https://www.postgresql.org/docs/current/transaction-iso.html
- https://www.postgresql.org/docs/current/explicit-locking.html
- https://www.postgresql.org/docs/current/errcodes-appendix.html
- https://www.postgresql.org/docs/current/functions-admin.html
- https://www.pgbouncer.org/features.html
- https://brandur.org/idempotency-keys
- https://www.postgresql.org/docs/current/sql-set.html
- https://www.postgresql.org/docs/current/sql-notify.html
- https://www.postgresql.org/docs/current/storage-hot.html
- https://www.postgresql.org/docs/current/hot-standby.html
