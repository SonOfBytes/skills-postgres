# Partitioning

Partitioning is a lifecycle tool first and a performance tool second. Most OLTP tables never need
it, and a surrogate-keyed table usually cannot have it without changing its keys.

## Whether

Partition when **both** hold [PG ddl-partitioning; GitLab partitioning]:

1. The table would otherwise exceed the server's physical memory (the documentation's own rule of
   thumb), and
2. either the partition key appears in the `WHERE` of nearly every query (so pruning applies), or
   retention deletes whole time ranges (so `DROP PARTITION` replaces a mass `DELETE` and its
   vacuum).

"The table is big" alone is not a reason: pages swap the same whether they sit in one table or
many; the unambiguous wins are fast partition drops and smaller hot-partition indexes [Percona
free lunches].

## The constraint that decides it

Every unique constraint and the primary key must include the partition key, because a
per-partition index can only enforce uniqueness within its partition; there are no global indexes
[PG ddl-partitioning]. A table keyed by `id` alone that you want to partition by `created_at`
needs `PRIMARY KEY (id, created_at)`, which every foreign key to it must then carry. This is the
step that usually ends the discussion for OLTP tables.

## How

- Declarative `RANGE`, `LIST` or `HASH`; never inheritance plus triggers [Percona traditional
  methods; wiki Don't Do This].
- Key = the column(s) most queries filter on; design so everything you drop at once lands in one
  partition.
- Keep the count in the low thousands at most; planning time and memory grow with partitions.
- A default partition as a safety net; `pg_partman` to create partitions on schedule [Crunchy 19;
  Percona pg_partman].
- Indexes created on the parent propagate to partitions.

```sql
CREATE TABLE watch_events (
  id         bigint GENERATED ALWAYS AS IDENTITY,
  user_id    uuid NOT NULL,
  video_id   uuid NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (id, created_at)
) PARTITION BY RANGE (created_at);

CREATE TABLE watch_events_2026_09 PARTITION OF watch_events
  FOR VALUES FROM ('2026-09-01') TO ('2026-10-01');
CREATE TABLE watch_events_default PARTITION OF watch_events DEFAULT;

CREATE INDEX watch_events_user_idx ON watch_events (user_id, created_at DESC);
```

## Operations

- `[pg>=14]` `ALTER TABLE … DETACH PARTITION … CONCURRENTLY` takes `SHARE UPDATE EXCLUSIVE` on the
  parent rather than `ACCESS EXCLUSIVE` [PG sql-altertable; squawk].
- `ATTACH PARTITION` takes `SHARE UPDATE EXCLUSIVE` on the parent; add a matching `CHECK` to the
  table first so the attach skips the validation scan [PG sql-altertable].
- Autovacuum does not analyse partitioned parents: `ANALYZE` the parent explicitly after loads;
  `[pg>=18]` `ANALYZE ONLY parent` [PG routine-vacuuming; PG 18 release notes].
- `[pg>=17]` identity columns work on partitioned tables; `[pg>=19]` `MERGE PARTITIONS` /
  `SPLIT PARTITION` DDL [Crunchy 19].
- Verify pruning in `EXPLAIN`: if every partition appears, the key is not in the predicate.

## Partitioning an existing large table

A multi-release project, not a migration: create the partitioned copy, install a trigger that
mirrors writes, copy in batches in the background, verify, swap names in one short transaction,
drop the old table later [GitLab partitioning]. See `migration-recipes.md` for the batch pattern.

## Sources

- https://www.postgresql.org/docs/current/ddl-partitioning.html
- https://docs.gitlab.com/development/database/partitioning/
- https://www.percona.com/blog/partitioning-free-lunches-and-indexing/
- https://www.percona.com/blog/postgresql-partitioning-using-traditional-methods/
- https://wiki.postgresql.org/wiki/Don%27t_Do_This
- https://www.crunchydata.com/blog/postgres-19-how-our-advice-has-changed-since-we-wrote-it
- https://www.percona.com/blog/postgresql-partitioning-made-easy-using-pg_partman-timebased/
- https://www.postgresql.org/docs/current/sql-altertable.html
- https://squawkhq.com/docs/rules
- https://www.postgresql.org/docs/current/routine-vacuuming.html
- https://www.postgresql.org/docs/release/18.0/
