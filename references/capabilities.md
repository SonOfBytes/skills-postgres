# Differentiating Capabilities

What Postgres does that would otherwise need another system, and how to use each without
regret. Queues are in `queues.md`, partitioning in `partitioning.md`.

## Full-text search

- A `tsvector` column `GENERATED ALWAYS AS (to_tsvector('english', …)) STORED` with a GIN index,
  not an expression index: queries need not repeat the configuration to use the index, and
  `to_tsvector` is not re-evaluated per row [PG textsearch-tables].
- Weight fields (`setweight(to_tsvector(title), 'A') || setweight(to_tsvector(body), 'B')`) and
  rank with `ts_rank_cd`. Pair with `pg_trgm` for typo tolerance; they solve different problems.

```sql
CREATE TABLE articles (
  id     bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  title  text NOT NULL,
  body   text NOT NULL,
  search tsvector GENERATED ALWAYS AS (
    setweight(to_tsvector('english', title), 'A') ||
    setweight(to_tsvector('english', body), 'B')
  ) STORED
);
CREATE INDEX articles_search_idx ON articles USING gin (search);

SELECT id, title, ts_rank_cd(search, query) AS rank
FROM articles, websearch_to_tsquery('english', $1) AS query
WHERE search @@ query
ORDER BY rank DESC
LIMIT 20;
```

## Trigram search

- `pg_trgm` with a GIN index (`gin_trgm_ops`) removes B-tree's left-anchoring requirement for
  `LIKE '%abc%'`, `ILIKE` and regex; GiST when you rank by `<->` distance. Patterns shorter than
  three characters produce no trigrams and fall back to a full index scan [PG pgtrgm].

```sql
CREATE EXTENSION IF NOT EXISTS pg_trgm;
CREATE INDEX channels_title_trgm_idx ON channels USING gin (title gin_trgm_ops);
SELECT id, title FROM channels WHERE title ILIKE '%' || $1 || '%' LIMIT 20;
```

## Vector search (pgvector)

- HNSW by default: better query performance for its build time and no training step; IVFFlat
  only when build time or memory is the binding constraint. Build after bulk `COPY`, raise
  `maintenance_work_mem`, and enable iterative scans when combining nearest-neighbour with a
  filter, or post-filtering returns too few rows [pgvector README].

```sql
CREATE EXTENSION IF NOT EXISTS vector;
CREATE TABLE embeddings (
  id        bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  doc_id    bigint NOT NULL,
  embedding vector(1536) NOT NULL
);
CREATE INDEX embeddings_hnsw_idx ON embeddings USING hnsw (embedding vector_cosine_ops);
SELECT doc_id FROM embeddings ORDER BY embedding <=> $1 LIMIT 10;
```

## Ranges and exclusion

Range types with a GiST exclusion constraint make double-booking structurally impossible; the
`SELECT`-then-`INSERT` check races. `[pg>=18]` temporal primary keys (`WITHOUT OVERLAPS`) express
the same natively. See `constraints.md` [PG rangetypes; PG 18 release notes].

## Materialized views

- A read model refreshed on a schedule. To stay readable during refresh, create a **full**
  unique index (column names only, no `WHERE`) and use `REFRESH MATERIALIZED VIEW CONCURRENTLY`;
  it is slower than a plain refresh but does not lock out readers. One refresh at a time [PG
  sql-refreshmaterializedview].

```sql
CREATE MATERIALIZED VIEW channel_stats AS
SELECT channel_id, count(*) AS video_count, max(published_at) AS latest
FROM videos
GROUP BY channel_id;
CREATE UNIQUE INDEX channel_stats_pkey ON channel_stats (channel_id);
REFRESH MATERIALIZED VIEW CONCURRENTLY channel_stats;
```

## Change data capture

- Logical decoding through a replication slot streams committed changes to a consumer. A slot
  retains WAL until consumed: an abandoned slot fills the disk. Bound it with
  `max_slot_wal_keep_size` and alert on slot lag. Set `REPLICA IDENTITY` deliberately: with the
  default, `UPDATE` old-row and `DELETE` images need a primary key; `FULL` gives whole images at a
  write cost [PG logicaldecoding; runtime-config-replication].
- `[pg>=16]` logical replication can be initiated from a standby.

## Generated columns, arrays, jsonb

Covered in `types.md`; the capability note is that `[pg>=16]` SQL/JSON constructors and
`[pg>=17]` `JSON_TABLE()` shred documents into rows without application code.

## Transactional DDL

Every migration runs inside a transaction, so a failed step leaves no half-applied schema.
Exceptions that cannot run in a transaction block: `CREATE INDEX CONCURRENTLY`, `REINDEX
CONCURRENTLY`, `VACUUM`, and `[pg>=19]` `REPACK`. See `migrations.md` [PG sql-createindex;
sql-reindex].

## Scheduling: pg_cron

- Runs SQL on a schedule inside the database; jobs are stored in one metadata database per
  cluster, capped by `max_worker_processes` when using background workers, and **no jobs run
  while the server is a hot standby**, so a failover silently stops them [pg_cron README]. A
  system scheduler calling `psql` is often simpler.

## Vetting an extension

Ask, before `CREATE EXTENSION`: does it run in-process as a shared library or background worker
(then it can crash the server); is it available on the managed provider you deploy to; what
happens on failover and on a standby; who maintains it and on what release cadence; does it
create functions in a writable schema (a `search_path` hazard, `security.md`). Install by the
migrator role and list every extension in the data-model document [Crunchy CIS checklist].

## Sources

- https://www.postgresql.org/docs/current/textsearch-tables.html
- https://www.postgresql.org/docs/current/pgtrgm.html
- https://github.com/pgvector/pgvector
- https://www.postgresql.org/docs/current/rangetypes.html
- https://www.postgresql.org/docs/release/18.0/
- https://www.postgresql.org/docs/current/sql-refreshmaterializedview.html
- https://www.postgresql.org/docs/current/logicaldecoding.html
- https://www.postgresql.org/docs/current/runtime-config-replication.html
- https://www.postgresql.org/docs/current/sql-createindex.html
- https://www.postgresql.org/docs/current/sql-reindex.html
- https://github.com/citusdata/pg_cron
- https://www.crunchydata.com/blog/postgres-security-checklist-from-the-center-for-internet-security
