# Queries

How to write SQL the planner can use and the next reader can trust. Pair with `indexing.md`
(what serves the predicate) and `transactions.md` (what happens when two run at once).

## Shape

- Explicit column lists. `SELECT *` pulls TOASTed values you did not want, blocks index-only
  scans, and changes shape under a migration [PG storage-toast].
- Every list query has a `LIMIT` or streams. Test databases are small; production is not [EF
  Core efficient querying].
- One definition per product rule. Two queries that must stay behaviourally identical (an
  ordering walk and its browse page) sit side by side, share the index, and carry a comment
  saying so.
- Parameter types match column types exactly (`$1::uuid`, `$2::timestamptz`); a cast, function
  or arithmetic on the **column** side of a predicate defeats the index. Index the expression
  if the transformation is the real predicate [use-the-index-luke obfuscation].

## Existence and anti-joins

- `EXISTS (SELECT 1 …)` for "is there any"; never `count(*) > 0` [PG functions-subquery].
- `NOT EXISTS`, never `NOT IN (subquery)`: one NULL in the subquery makes `NOT IN` return no
  rows, and the planner cannot form an anti-join [PG functions-subquery; wiki Don't Do This].

```sql
SELECT v.id
FROM videos v
WHERE NOT EXISTS (
  SELECT 1 FROM video_blocks b
  WHERE b.household_id = $1 AND b.video_id = v.id
);
```

## Lists of values

- One array parameter, not a generated `IN ($1, $2, …)` list: one plan-cache entry instead of
  one per list length, and no 65535-parameter ceiling [sqlc select howto].

```sql
SELECT id, title FROM videos WHERE id = ANY($1::uuid[]);
```

## Pagination

- Keyset (seek) pagination on any list that grows: a row-value comparison on the sort tuple,
  served by an index on the same tuple. `OFFSET n` fetches and discards `n` rows every page and
  skips or repeats rows when the list changes underneath [EF Core efficient querying; PG
  queries-limit].
- The tuple must be unique (`(published_at, id)`, never `published_at` alone) or ties make the
  walk skip rows; the comparison is a **row-value** comparison, not two ANDed predicates [PG
  functions-comparisons].

```sql
SELECT id, title, published_at
FROM videos
WHERE channel_id = $1
  AND ($2::timestamptz IS NULL OR (published_at, id) > ($2, $3::uuid))
ORDER BY published_at ASC, id ASC
LIMIT $4;
```

## Upserts

- `INSERT … ON CONFLICT (cols) DO UPDATE SET …` with an explicit conflict target: an atomic
  insert-or-update outcome is guaranteed even under high concurrency [PG sql-insert].
- A multi-row upsert may not touch the same existing row twice (cardinality violation):
  de-duplicate the input first [PG sql-insert].
- To target a partial unique index, repeat its predicate: `ON CONFLICT (video_id) WHERE state =
  'ready'` [PG sql-insert].
- Expect sequence gaps: the default is evaluated before conflict detection [wiki Don't Do This].
- `[pg>=15]` `MERGE` is for multi-action, source-driven synchronisation; it obeys ordinary
  isolation and can raise a unique violation under a concurrent insert, so it is not a drop-in
  concurrent upsert [PG sql-merge]. `[pg>=17]` `MERGE … RETURNING` with `merge_action()`.

```sql
INSERT INTO channel_approvals (household_id, channel_id, status, approved_by)
VALUES ($1, $2, 'active', $3)
ON CONFLICT (household_id, channel_id) DO UPDATE
  SET status = EXCLUDED.status,
      updated_at = now()
RETURNING household_id, channel_id, status;
```

## RETURNING

- Return generated ids, defaults and computed values from the writing statement instead of a
  follow-up `SELECT`. `[pg>=18]` `RETURNING OLD.col, NEW.col` gives the pre-image too [PG 18
  release notes].

## CTEs, LATERAL, DISTINCT ON

- A CTE referenced once is inlined (since 12); write `MATERIALIZED` only as a deliberate
  optimisation fence, with a comment. A data-modifying CTE's effects are invisible to its
  siblings: read its `RETURNING`, not the base table [PG queries-with].
- `LATERAL` for a correlated per-row subquery: top-N per group, the latest child, "the one
  asset to show for this video".
- `DISTINCT ON (key) … ORDER BY key, tiebreak` for one row per key; without the `ORDER BY` the
  row chosen is unpredictable [PG sql-select].

```sql
SELECT v.id, v.title, a.state
FROM videos v
LEFT JOIN LATERAL (
  SELECT m.state
  FROM media_assets m
  WHERE m.video_id = v.id AND m.state IN ('ready', 'packaging')
  ORDER BY (m.state = 'ready') DESC
  LIMIT 1
) a ON true
WHERE v.channel_id = $1;
```

## Counting

- `count(*)` over a table is proportional to the table. A UI badge takes an estimate
  (`pg_class.reltuples`) or a maintained counter; an exact count is for a bounded predicate [PG
  functions-aggregate].

## Bulk

- `COPY` for loads, inside one transaction, indexes created afterwards, `ANALYZE` at the end;
  `[pg>=17]` `ON_ERROR ignore` and `LOG_VERBOSITY` for dirty input; `[pg>=18]` `REJECT_LIMIT`
  [PG populate; PG 17 release notes; PG 18 release notes].
- Batches of independent statements in one round trip (pgx `SendBatch`, Npgsql `NpgsqlBatch`);
  never a loop of single-row statements over the network [driver references].

## Reading a plan

`EXPLAIN (ANALYZE, BUFFERS)` — always `BUFFERS`; `[pg>=18]` it is included automatically.
`track_io_timing = on` makes the I/O numbers meaningful [wiki Slow Query Questions].

1. Find the lowest node where estimated rows and actual rows differ by ten times or more; a
   misestimate there is what chose the wrong join above it [Cybertec explain].
2. Multiply a node's time by its `loops`; upper-node times are cumulative, so subtract children
   to get a node's own cost [Cybertec explain].
3. `shared hit` is cache, `shared read` is disk; a cold container reads everything [pganalyze
   planning basics].
4. `Heap Fetches` on an Index Only Scan says how often the visibility map failed you.
5. Prepared statements switch to a generic plan after five executions; a query that regresses on
   the sixth call with skewed parameters wants `SET LOCAL plan_cache_mode = force_custom_plan`
   [PG sql-prepare; runtime-config-query].
6. Stale statistics are the most common cause of a bad plan: `ANALYZE` the table and look again
   before adding an index.

## Statistics

- `ANALYZE` after bulk loads and explicitly on partitioned parents [PG routine-vacuuming].
- Columns that are filtered together and correlated (city and postcode, state and kind) get
  `CREATE STATISTICS … (dependencies, ndistinct)`; raise a column's statistics target where
  estimates stay wrong [PG planner-stats].

```sql
CREATE STATISTICS videos_channel_kind_stats (dependencies, ndistinct)
  ON channel_id, kind FROM videos;
```

## Sources

- https://www.postgresql.org/docs/current/storage-toast.html
- https://learn.microsoft.com/en-us/ef/core/performance/efficient-querying
- https://use-the-index-luke.com/sql/where-clause/obfuscation
- https://www.postgresql.org/docs/current/functions-subquery.html
- https://wiki.postgresql.org/wiki/Don%27t_Do_This
- https://docs.sqlc.dev/en/stable/howto/select.html
- https://www.postgresql.org/docs/current/queries-limit.html
- https://www.postgresql.org/docs/current/functions-comparisons.html
- https://www.postgresql.org/docs/current/sql-insert.html
- https://www.postgresql.org/docs/current/sql-merge.html
- https://www.postgresql.org/docs/release/18.0/
- https://www.postgresql.org/docs/release/17.0/
- https://www.postgresql.org/docs/current/queries-with.html
- https://www.postgresql.org/docs/current/sql-select.html
- https://www.postgresql.org/docs/current/functions-aggregate.html
- https://www.postgresql.org/docs/current/populate.html
- https://wiki.postgresql.org/wiki/Slow_Query_Questions
- https://www.cybertec-postgresql.com/en/how-to-interpret-postgresql-explain-analyze-output/
- https://pganalyze.com/docs/explain/basics-of-postgres-query-planning
- https://www.postgresql.org/docs/current/sql-prepare.html
- https://www.postgresql.org/docs/current/runtime-config-query.html
- https://www.postgresql.org/docs/current/routine-vacuuming.html
- https://www.postgresql.org/docs/current/planner-stats.html
