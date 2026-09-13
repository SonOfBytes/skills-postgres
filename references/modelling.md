# Modelling: Entities, Tenancy, Polymorphism, Deletion, State, History

The decisions that shape every query afterwards. Each pattern below names the shape to use, the
shape to avoid, and why.

## Normalise first, denormalise on measurement

- Attributes that are queried, joined, constrained or indexed are columns on the table that owns
  them. Denormalise only after a measured problem, and prefer schema and index work first: "a
  thoughtfully designed schema with well-chosen indexes often provides a bigger boost" than
  tuning tricks [pganalyze hints].
- A denormalised copy needs a documented maintenance path: a trigger, the upsert that writes
  both, or a batch job. Without one it drifts.
- Never Entity-Attribute-Value: one insert becomes four rows across three tables with four index
  updates, and a seven-line query becomes an eighteen-line join. Use columns plus a `jsonb`
  overflow for the rare attributes [Cybertec EAV].

## Shared versus segregated data

Decide, per table, whether it is **global** (shared by every tenant or user: a catalogue, media,
reference data) or **scoped** (curation, progress, settings, anything one party controls). Write
the decision in the data-model document, because a global table in the wrong place leaks and a
scoped table in the wrong place duplicates work. Global tables carry no tenant column and no
tenant filter; scoped tables carry the tenant column everywhere (below).

## Multi-tenancy

Shape by tenant count [Crunchy multi-tenancy]:

| Tenants | Shape | Trade-off |
|---|---|---|
| tens | database per tenant | maximal isolation; N schemas to migrate; connection count multiplies |
| hundreds | schema per tenant | cross-tenant analytics still possible; migrations run N times; `search_path` becomes security-relevant |
| thousands to millions | shared tables with a tenant column | scales; isolation is a discipline enforced by constraints, queries, tests and RLS |

Shared tables, the discipline [Citus row-based sharding; Supabase RLS performance]:

- The tenant column is on **every** scoped table, first in the primary key and in every scoped
  unique constraint, and part of every scoped foreign key, so a child row cannot point at
  another tenant's parent.
- Every scoped query takes the tenant id as an explicit parameter and joins or filters on it;
  never derive it from a session variable alone.
- A two-tenant leak test exists for every scoped read (see `testing.md`).
- Row-level security is defence in depth behind all of the above (see `rls.md`), and the column
  every policy filters on is indexed.

```sql
CREATE TABLE video_blocks (
  household_id uuid NOT NULL REFERENCES households (id) ON DELETE CASCADE,
  video_id     uuid NOT NULL REFERENCES videos (id) ON DELETE CASCADE,
  blocked_at   timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (household_id, video_id)
);

-- Scoped read: the tenant is a parameter and a join condition, not an afterthought.
SELECT v.id, v.title
FROM videos v
JOIN channel_approvals ca
  ON ca.channel_id = v.channel_id AND ca.household_id = $1 AND ca.status = 'active'
WHERE NOT EXISTS (
  SELECT 1 FROM video_blocks b WHERE b.household_id = $1 AND b.video_id = v.id
);
```

## Polymorphic references

- The Rails-style pair (`resource_type text, resource_id bigint`) has no referential integrity;
  the database cannot check it and you reimplement foreign keys in code [GitLab polymorphic].
- For a handful of target types, the exclusive arc: one nullable foreign key per target and a
  `CHECK` that exactly one is set. NULLs are near-free, every reference is a real foreign key,
  and adding a type is one migration [GitLab polymorphic].
- Beyond about three targets, a supertype table (`attachables`) that every concrete type
  references, so the polymorphic side points at one table.

```sql
CREATE TABLE comments (
  id         bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  video_id   uuid REFERENCES videos (id) ON DELETE CASCADE,
  channel_id uuid REFERENCES channels (id) ON DELETE CASCADE,
  body       text NOT NULL,
  CHECK (num_nonnulls(video_id, channel_id) = 1)
);
```

## Deletion

- Default to hard delete. A reflexive `deleted_at` leaks `WHERE deleted_at IS NULL` into every
  query (one omission exposes data), keeps "deleted" parents referenced by live children, and
  makes real erasure a multi-table exercise; across a decade at Heroku and Stripe the author
  never saw undelete used [Brandur soft deletion; Postgres.fm soft delete].
- When recoverability or audit is required, move the row: one `deleted_record` table with the
  original as `jsonb`, purged by a retention job. Live tables stay clean and foreign keys stay
  honest [Brandur soft deletion].
- If soft delete is nonetheless the project's convention: partial indexes `WHERE deleted_at IS
  NULL` on every hot path, unique constraints that include or exclude deleted rows deliberately,
  and a view or a store-layer rule that applies the predicate once.

```sql
CREATE TABLE deleted_record (
  id             bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  deleted_at     timestamptz NOT NULL DEFAULT now(),
  original_table text NOT NULL,
  original_id    text NOT NULL,
  data           jsonb NOT NULL
);
```

## State machines

- A `status` column with a closed value set (see `types.md`), and the transitions enforced in
  **one** place: a single service method, or a trigger, or a `CHECK` on a transition table.
  Transitions scattered across handlers are how a job reaches `done` twice.
- When history matters, an append-only transition or event table beside the current-state
  column; the current state is a projection of the log, and the log is what audit reads.
- For resumable multi-step work, store a **recovery point** (`started`, `charge_created`,
  `finished`) so a retry resumes from where it stopped rather than repeating the side effects
  [Brandur idempotency keys]. The idempotency key that guards it is `UNIQUE (scope_id, key)`;
  the protocol is in `transactions.md`.

## History and time

- Postgres has no system versioning. History is a trigger-maintained history table, an event
  table, or an extension; `[pg>=18]` temporal primary keys cover *application* time (validity
  periods), not automatic row versioning [PG 18 release notes].
- Validity periods use a range column with an exclusion constraint (see `constraints.md`).

## Reference and lookup data

- Small, stable reference sets that never need attributes are a `CHECK` or an enum; reference
  data with attributes (country names, plans) is a table, seeded by a migration or a seed file
  the tests also use.

## Single-row settings tables

- A global settings row uses a primary key that can only be `true`, so a second row is
  impossible and the table reads without a `WHERE` [common idiom; constraint semantics from PG
  ddl-constraints].

```sql
CREATE TABLE settings (
  id                 boolean PRIMARY KEY DEFAULT true CHECK (id),
  cache_budget_bytes bigint NOT NULL DEFAULT 64424509440,
  updated_at         timestamptz NOT NULL DEFAULT now()
);
```

## What must not live in the database

Ask of every column: is it derived, transient, or a secret? Resolved URLs that expire, parsed
caches that can be refetched, process-local health state, in-flight progress written at high
frequency, raw third-party responses, and plaintext tokens all belong elsewhere (a cache, memory,
object storage, or as a hash). Writing them to Postgres is churn at best and a breach at worst
[OWASP database security].

## Personal data

- Know which columns hold personal data; comment them. Every such table has a retention rule and
  an erasure path that works through the foreign-key graph (an audit table with `ON DELETE
  RESTRICT` to a person makes erasure a migration) [OWASP database security].

## Sources

- https://pganalyze.com/blog/migrating-from-oracle-hints-to-pg-hint-plan-on-postgresql
- https://www.cybertec-postgresql.com/en/entity-attribute-value-eav-design-in-postgresql-dont-do-it/
- https://www.crunchydata.com/blog/designing-your-postgres-database-for-multi-tenancy
- https://www.citusdata.com/blog/2023/07/31/schema-based-sharding-comes-to-postgres-with-citus/
- https://supabase.com/docs/guides/troubleshooting/rls-performance-and-best-practices-Z5Jjwv
- https://docs.gitlab.com/development/database/polymorphic_associations.html
- https://brandur.org/soft-deletion
- https://postgres.fm/episodes/soft-delete
- https://brandur.org/idempotency-keys
- https://www.postgresql.org/docs/release/18.0/
- https://www.postgresql.org/docs/current/ddl-constraints.html
- https://cheatsheetseries.owasp.org/cheatsheets/Database_Security_Cheat_Sheet.html
