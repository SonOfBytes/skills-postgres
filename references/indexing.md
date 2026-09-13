# Indexing

An index is a trade: faster reads for that predicate against slower writes for every row. Add
one for a query you can name; verify it with `EXPLAIN (ANALYZE, BUFFERS)`; remove it when nothing
uses it. Invariant indexes (unique, partial unique, exclusion) are in `constraints.md`; building
an index on a live table is in `migration-recipes.md`.

## Choosing the access method

| Need | Method | Notes |
|---|---|---|
| equality, range, sort, `IN`, `IS NULL`, left-anchored `LIKE 'abc%'` | B-tree (default) | the workhorse; everything else is a specialisation [PG indexes-types] |
| `jsonb` containment `@>`, `@?`, `@@` | GIN `jsonb_path_ops` | smaller and faster than `jsonb_ops`; no index entries for empty structures [PG datatype-json] |
| `jsonb` key existence `?`, `?|`, `?&` | GIN `jsonb_ops` | only when you need those operators [PG datatype-json] |
| one scalar `jsonb` key in equality or range predicates | B-tree on the expression `((doc->>'status'))` | GIN cannot do range or index-only scans [PG indexes-index-only-scans] |
| leading-wildcard `LIKE '%abc%'`, `ILIKE`, regex, fuzzy | GIN `gin_trgm_ops` (GiST for `<->` distance ranking) | patterns under three characters degrade to a full scan [PG pgtrgm] |
| full-text search | GIN on a `tsvector` column | see `capabilities.md` |
| ranges, geometry, nearest-neighbour `ORDER BY col <-> point` | GiST | backs exclusion constraints too [PG indexes-types] |
| large append-only tables where the column follows physical order | BRIN | `[pg>=14]` `minmax_multi` tolerates imperfect order; useless after heavy updates [PG indexes-types; Crunchy 19] |
| vector similarity | HNSW (pgvector) | see `capabilities.md` |
| equality on a wide key, measured to beat B-tree | hash | cannot sort, cannot be unique, no index-only scan [PG indexes-types] |

## Composite indexes

- Equality columns first, then the single range or sort column last. Columns after the range
  column are filtered inside the index but do not narrow the scan [PG indexes-multicolumn;
  pganalyze how-postgres-chooses-index].
- Three columns or fewer unless you can name the exact query; wider non-unique indexes rarely
  help [PG indexes-multicolumn; strong_migrations].
- Match the `ORDER BY`: an index on `(channel_id, published_at, id)` serves both `WHERE channel_id
  = $1 ORDER BY published_at, id` and the keyset predicate `(published_at, id) > ($2, $3)`; that
  is why one index can carry an ordering rule and its pagination.
- `[pg>=18]` skip scan lets a composite index serve a query that omits a low-cardinality leading
  column, so one well-designed index can replace a companion; re-audit before adding a second
  [PG 18 release notes; Crunchy 19].
- `[pg>=17]` B-tree handles `IN (…)` lists of constants efficiently; do not rewrite them as
  `UNION ALL` [PG 17 release notes].

```sql
-- One index for the ordering rule, the keyset walk and the browse page.
CREATE INDEX videos_series_idx
  ON videos (channel_id, published_at ASC, id ASC)
  WHERE availability = 'public';
```

## Partial indexes

- Whenever the query only ever touches a small, statically describable subset: a queue's
  `WHERE state = 'queued'`, a soft-delete's `WHERE deleted_at IS NULL`, a nullable foreign key
  that is mostly NULL (769 MB to 5 MB in Haki Benita's case; Postgres indexes NULLs) [PG
  indexes-partial; Haki Benita].
- The query's predicate must imply the index predicate or the planner will not use it; keep the
  two textually identical.

```sql
CREATE INDEX package_jobs_claim_idx
  ON package_jobs (priority, not_before)
  WHERE state = 'queued';
```

## Covering indexes and index-only scans

- Add payload columns with `INCLUDE`, not as key columns: uniqueness still applies to the key
  only, and `INCLUDE` columns need no operator class. Cost: a bigger index and no B-tree
  deduplication [PG indexes-index-only-scans; sql-createindex].
- An index-only scan happens only when the visibility map says the page is all-visible; on a
  hot, frequently updated table the scan still touches the heap. Check `Heap Fetches` in the
  plan before promising one [PG indexes-index-only-scans].
- For an index-only scan over an expression index, include the base column: `ON tab (f(x))
  INCLUDE (x)` [PG indexes-index-only-scans].

```sql
CREATE INDEX sessions_lookup_idx
  ON sessions (id)
  INCLUDE (user_id, expires_at);
```

## Foreign-key columns

Index the referencing column of every foreign key unless the child table is small, the parent
key is never deleted or updated, and you never join on it. Postgres does not create it; a parent
`DELETE` otherwise scans the child (154 ms against 0.75 ms) [Cybertec FK index]. Catalog query to
find the gaps:

```sql
SELECT c.conrelid::regclass AS "table", c.conname
FROM pg_constraint c
WHERE c.contype = 'f'
  AND NOT EXISTS (
    SELECT 1 FROM pg_index i
    WHERE i.indrelid = c.conrelid
      AND (i.indkey::int2[])[0:array_length(c.conkey, 1) - 1] = c.conkey
  );
```

## Write cost and HOT updates

- An update that changes no indexed column and finds space on the page is a HOT update: no new
  index entries, dead versions pruned without vacuum. Every index on an updated column removes
  that; lower `fillfactor` (70 to 90) on update-heavy tables to keep page space, and watch
  `n_tup_hot_upd` in `pg_stat_user_tables` [PG storage-hot].
- Do not create an index for a one-off query, and do not create every index a planner tool
  suggests: "modifying an indexed table can easily be an order of magnitude more expensive"
  [Crunchy indexes for newbies; pganalyze deconstructing the planner].

## Unused, duplicate and invalid indexes

- Unused: `pg_stat_user_indexes` with `idx_scan < 10` (not `= 0`, to spare a rare but critical
  index), excluding unique, primary-key and FK-supporting indexes, measured over a full business
  cycle and on replicas separately [Cybertec unused indexes; PG monitoring-stats].
- Duplicate: an index on `(a)` next to `(a, b)` is usually redundant, unless skip scan or a
  unique constraint distinguishes them.
- Invalid: a `CREATE INDEX CONCURRENTLY` that failed leaves `indisvalid = false`; the index is
  maintained on every write and never used. Drop and rebuild [PG sql-createindex].

```sql
SELECT indexrelid::regclass AS index_name
FROM pg_index
WHERE NOT indisvalid;
```

## Building and rebuilding on a live table

- `CREATE INDEX CONCURRENTLY`, outside a transaction; `REINDEX INDEX … CONCURRENTLY` for bloat,
  then check for `_ccnew` leftovers. Details in `migration-recipes.md` [PG sql-createindex;
  sql-reindex].
- `[pg>=19]` `REPACK (CONCURRENTLY)` rewrites a bloated table online without `pg_repack`
  [Crunchy 19].

## Verifying

- `EXPLAIN (ANALYZE, BUFFERS)` with production-like volume; on a ten-row test table the planner
  rightly chooses a seq scan, so an index assertion is meaningless without seeding past the
  crossover [pganalyze planning basics]. Reading plans is in `queries.md`.

## Sources

- https://www.postgresql.org/docs/current/indexes-types.html
- https://www.postgresql.org/docs/current/datatype-json.html
- https://www.postgresql.org/docs/current/indexes-index-only-scans.html
- https://www.postgresql.org/docs/current/pgtrgm.html
- https://www.crunchydata.com/blog/postgres-19-how-our-advice-has-changed-since-we-wrote-it
- https://www.postgresql.org/docs/current/indexes-multicolumn.html
- https://pganalyze.com/blog/how-postgres-chooses-index
- https://github.com/ankane/strong_migrations
- https://www.postgresql.org/docs/release/18.0/
- https://www.postgresql.org/docs/release/17.0/
- https://www.postgresql.org/docs/current/indexes-partial.html
- https://hakibenita.com/postgresql-unused-index-size
- https://www.postgresql.org/docs/current/sql-createindex.html
- https://www.cybertec-postgresql.com/en/index-your-foreign-key/
- https://www.postgresql.org/docs/current/storage-hot.html
- https://www.crunchydata.com/blog/postgres-indexes-for-newbies
- https://pganalyze.com/blog/deconstructing-the-postgres-planner
- https://www.cybertec-postgresql.com/en/get-rid-of-your-unused-indexes/
- https://www.postgresql.org/docs/current/monitoring-stats.html
- https://www.postgresql.org/docs/current/sql-reindex.html
- https://pganalyze.com/docs/explain/basics-of-postgres-query-planning
