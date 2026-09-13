# Job Queues in Postgres

A queue table with `FOR UPDATE SKIP LOCKED` is correct, transactional with your data, and good to
a surprising volume. It also has one failure mode that takes it from milliseconds to seconds
without warning. Build the claim, the lease, and the guard together.

## The claim

```sql
CREATE TABLE package_jobs (
  id               bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  video_id         uuid NOT NULL REFERENCES videos (id) ON DELETE CASCADE,
  priority         smallint NOT NULL DEFAULT 50,
  state            text NOT NULL DEFAULT 'queued'
                     CHECK (state IN ('queued', 'leased', 'done', 'failed', 'dead')),
  attempts         int NOT NULL DEFAULT 0,
  not_before       timestamptz NOT NULL DEFAULT now(),
  leased_by        text,
  lease_expires_at timestamptz,
  last_error       text,
  created_at       timestamptz NOT NULL DEFAULT now(),
  updated_at       timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX package_jobs_active_idx ON package_jobs (video_id)
  WHERE state IN ('queued', 'leased');
CREATE INDEX package_jobs_claim_idx ON package_jobs (priority, not_before)
  WHERE state = 'queued';
CREATE INDEX package_jobs_lease_idx ON package_jobs (lease_expires_at)
  WHERE state = 'leased';
```

```sql
UPDATE package_jobs SET
  state = 'leased',
  leased_by = $1,
  lease_expires_at = now() + $2::interval,
  attempts = attempts + 1,
  updated_at = now()
WHERE id IN (
  SELECT id FROM package_jobs
  WHERE state = 'queued' AND not_before <= now()
  ORDER BY priority ASC, not_before ASC
  LIMIT $3
  FOR UPDATE SKIP LOCKED
)
RETURNING id, video_id, priority, attempts;
```

- `SKIP LOCKED` lets N workers claim distinct rows without blocking each other [PG
  explicit-locking].
- The partial index matches the claim predicate **and** its `ORDER BY`, so the claim is an index
  walk from the front.
- The lease has an expiry; a worker that dies loses the lease and another picks the job up. Make
  the work restartable from zero rather than resumable, unless resume is cheap and provably
  correct.
- One active job per subject (`package_jobs_active_idx`) so a retry cannot double-enqueue.

## The failure mode

Any long-running transaction anywhere in the database holds the xmin horizon; vacuum cannot
remove the dead tuples that every claim and completion leaves under the claim index; the claim
walks dead entries before it finds a live row. Brandur measured lock times rising fifteen-fold
once dead tuples passed about 100,000 [Brandur postgres-queues]. Guards:

- A narrow claim predicate that skips the dead region (`WHERE id > $last_worked_id` when ids
  are monotonic, or a `not_before` floor).
- `statement_timeout` on worker roles and `idle_in_transaction_session_timeout` everywhere
  (`operations.md`).
- Monitoring of the oldest transaction age and dead tuples on the queue table; a per-table
  `autovacuum_vacuum_scale_factor` of 0.01 or lower.
- Done rows moved or deleted promptly (a `done` row is dead weight under the partial indexes'
  siblings and bloat in the heap).

## Global concurrency limits

`SKIP LOCKED` gives per-row exclusivity, not "at most N of this kind running". For that, layer a
transaction-scoped advisory lock keyed on the kind, or a counter row updated in the same
transaction as the claim [PG functions-admin; PG explicit-locking].

## Enqueueing from a transaction

Never push to an external broker inside a database transaction: either the worker starts before
the enclosing transaction commits (and cannot see the data), or the process dies after commit and
the job is silently lost. Stage the job in a table in the same transaction; a drainer moves it to
the broker after commit [Brandur job-drain]. A Postgres-only queue is this pattern with the
drainer removed.

## Waking workers

Poll with a short interval and jitter; add `LISTEN`/`NOTIFY` only as an accelerator, because it
delivers nothing to a worker that is not connected at commit and does not pass through
transaction pooling (`transactions.md`).

## Priorities and delays

`priority` (lower first) plus `not_before` covers "child is waiting", "next episode", "new
upload", "background window" without separate tables. Exponential back-off is `not_before = now()
+ interval '1 minute' * power(2, attempts)`; after a bounded number of attempts the job goes to
`dead` and a human view surfaces it, so a permanently failing subject cannot block the queue
behind it.

## Testing

Two real connections: worker A claims, worker B must not receive the same row; A's lease expires,
B claims it. A single connection tests nothing here (`testing.md`).

## Sources

- https://www.postgresql.org/docs/current/explicit-locking.html
- https://brandur.org/postgres-queues
- https://www.postgresql.org/docs/current/functions-admin.html
- https://brandur.org/job-drain
