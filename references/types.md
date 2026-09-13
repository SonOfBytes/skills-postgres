# Column Types, Keys and Value Sets

The choices that are cheap on day one and expensive to change on day four hundred. Rules are
"do X because Y [source]"; the sources are at the end.

## Text

- Use `text` by default, never `char(n)`: `char(n)` pads to width, makes trailing spaces
  significant in comparisons, and offers no storage or speed advantage; `text`, `varchar` and
  `varchar(n)` store identically [wiki Don't Do This; depesz].
- Do not add a `varchar(n)` limit "just in case"; a real domain limit goes in a `CHECK`, which
  can also enforce format and can be changed without a type change [wiki Don't Do This]. An
  unbounded `text` on a user-supplied field with no `CHECK` at all invites 100 MB names; state
  the bound the domain implies.
- Fixed-length codes: `text` plus `CHECK (length(code) = 3)`, because `char(3)` silently pads
  short values rather than rejecting them [wiki Don't Do This].

```sql
CREATE TABLE prices (
  id            bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  currency_code text NOT NULL CHECK (currency_code ~ '^[A-Z]{3}$'),
  display_name  text NOT NULL CHECK (length(display_name) BETWEEN 1 AND 200)
);
```

## Time

- Always `timestamptz`; plain `timestamp` is a wall-clock picture with no zone and cannot be
  converted correctly later, even if "we store UTC" [wiki Don't Do This].
- Never `timetz` or `CURRENT_TIME`; never `timestamp(0)` or `timestamptz(0)`, which round
  rather than truncate (a value up to half a second in the future). Use `date_trunc('second',
  now())` [wiki Don't Do This].
- Range predicates: `>= AND <`, not `BETWEEN`, which is a closed interval and double-counts the
  boundary instant [wiki Don't Do This].
- Durations: `interval` for lengths of time; a `bigint` of seconds or milliseconds when arithmetic
  or an external API dominates. Say which in the column comment.

## Numbers and money

- Money: `numeric` when fractional units or currency conversion occur; `bigint` minor units for a
  strictly single-currency system where all amounts are whole cents. Never `money` (bound to the
  cluster's `lc_monetary`, no fractional cents) and never `float4`/`float8` (binary rounding)
  [wiki Don't Do This; Crunchy money].
- Store the ISO 4217 currency code beside every amount when currencies can vary; never infer it
  from configuration [Crunchy money].
- `numeric(p, s)` with an explicit scale for anything that rounds at a boundary (`numeric(10,3)`
  for media offsets, `numeric(19,4)` for money); unconstrained `numeric` where precision is
  genuinely open.

## Surrogate keys

- `bigint GENERATED ALWAYS AS IDENTITY` for surrogate keys. Not `serial` (ownership and
  permission quirks); not `int` (2.1 billion, and alignment makes `bigint` free next to any
  8-byte column) [wiki Don't Do This; Crunchy serials].
- `ALWAYS` rather than `BY DEFAULT` unless an import or a dual-write migration must supply ids;
  `ALWAYS` prevents an application-supplied id from desynchronising the sequence [PG
  sql-altertable].
- Expect gaps: `ON CONFLICT` consumes a value even when it does not insert. Never expose an
  identity value as a gapless business number; keep a separate numbered column with its own
  rule [PG sql-insert].

```sql
CREATE TABLE orders (
  id         bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  created_at timestamptz NOT NULL DEFAULT now()
);
```

## UUID keys

- If keys must be generated outside the database or must not reveal ordering or volume, use
  UUID **version 7**: its timestamp prefix keeps B-tree inserts local; version 4 fragments every
  leaf page and costs roughly a quarter more index space [PG 18 release notes; RFC 9562].
- `[pg>=18]` `uuidv7()` is built in; `gen_random_uuid()` remains version 4. On older majors,
  generate v7 in the application (Go `github.com/google/uuid` `NewV7`, .NET `Guid.CreateVersion7`).
- Keep the natural external identifier (a provider's `UC…` channel id, an `ISBN`) as a unique
  `text` column beside the surrogate when it is "the real identity"; a natural primary key is
  fine only when it is immutable and narrow (see `constraints.md`).

```sql
CREATE TABLE channels (
  id                 uuid PRIMARY KEY,
  youtube_channel_id text NOT NULL UNIQUE,
  created_at         timestamptz NOT NULL DEFAULT now()
);
```

## Booleans and NULL

- Booleans `NOT NULL DEFAULT false` unless three-valued logic is genuinely modelled and
  documented; a `CHECK` cannot substitute because a `CHECK` passes when its expression is NULL
  [PG ddl-constraints].
- Prefer `NOT NULL` to `CHECK (col IS NOT NULL)`: cheaper, and `[pg>=18]` nameable and addable
  as `NOT VALID` [PG ddl-constraints; PG 18 release notes].
- Read `NULL` as "unknown", not "empty": an empty string default (`DEFAULT ''`) with `NOT NULL`
  is the right shape for "no value yet, and every consumer may treat it as text".

## Closed value sets

Choose by how the set evolves, and write the choice down once [Cybertec lookup-or-enum].

| Need | Use | Why |
|---|---|---|
| Short set, stable, additive changes | `text` + `CHECK (status IN (…))` | one migration to add a value; readable in every tool; no join |
| Values never removed; best planner estimates; many rows | native `ENUM` | 4-byte storage; a rename is a catalog change; **dropping a value is not implemented**, so removal is an additive multi-step [strong_migrations] |
| Values must be deletable or carry attributes | lookup table with a foreign key | fully flexible; costs a join and produces poor row estimates |

Do not mix strategies across sibling columns without a reason.

```sql
CREATE TABLE package_jobs (
  id    bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  state text NOT NULL DEFAULT 'queued'
          CHECK (state IN ('queued', 'leased', 'done', 'failed', 'dead'))
);
```

## JSON

- `jsonb`, never `json`: `json` stores text, reparses on every access and has no equality
  operator (`SELECT DISTINCT` breaks) [strong_migrations].
- The boundary: attributes that are queried, joined, constrained or indexed are columns. Rare,
  unconstrained, equality-only attributes may share one `jsonb` column with a GIN index. Never
  Entity-Attribute-Value tables [Cybertec EAV].
- Index the keys you query, not the whole column (see `indexing.md`) [Crunchy jsonb indexing].
- `[pg>=16]` `IS JSON`, JSON constructors; `[pg>=17]` `JSON_TABLE()` for shredding into rows.

## Arrays

- Arrays are not sets. A column you search by element or constrain per element is a child table;
  declared sizes (`int[3]`) are not enforced [PG arrays]. Arrays are right for a list that is
  always read whole (tags on a row that are displayed, never joined) and for passing id lists as
  a single parameter (`= ANY($1)`, see `queries.md`).

## Generated columns

- Expressions must be immutable and may not reference other rows, other generated columns,
  defaults or identity [PG ddl-generated-columns].
- `[pg>=18]` the default is `VIRTUAL` (computed on read, no storage); write the keyword
  explicitly so a migration behaves the same on every major. Use `STORED` when the column is
  indexed, partitioned on, uses a user-defined function or is expensive [PG
  ddl-generated-columns].
- Adding a `STORED` generated column to a large live table rewrites it (see
  `migration-recipes.md`) [strong_migrations].

```sql
CREATE TABLE articles (
  id     bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  title  text NOT NULL,
  body   text NOT NULL,
  search tsvector GENERATED ALWAYS AS (to_tsvector('english', title || ' ' || body)) STORED
);
```

## Binary and large values

- `bytea` for small binary values (hashes, tokens' digests). Media and documents go to object
  storage with a key in the row; the database is not a blob store [PG storage-toast].
- TOAST engages above about 2 KB per row and caps a value at 1 GB; wide `text` or `jsonb` that
  is substringed often benefits from `ALTER TABLE … ALTER COLUMN … SET STORAGE EXTERNAL` [PG
  storage-toast].

## Column order

- On a new table, order columns largest fixed-width first (`bigint`, `timestamptz`, `uuid`),
  then 4-byte (`int`, `real`, `date`), then 2-byte, then 1-byte (`boolean`), then variable-length
  (`text`, `jsonb`, `numeric`). Alignment padding is real: GitLab went from 48 to 40 bytes per
  row on an 80-million-row table [GitLab ordering columns]. Never reorder an existing table for
  this.

## Encoding

- Databases are `UTF8`; `SQL_ASCII` performs no conversion and leaves a mixture of encodings
  with no way back [wiki Don't Do This].

## Sources

- https://wiki.postgresql.org/wiki/Don%27t_Do_This
- https://www.depesz.com/2021/11/19/does-varcharn-use-less-disk-space-than-varchar-or-text/
- https://www.crunchydata.com/developers/playground/working-with-money-in-postgres
- https://www.crunchydata.com/blog/postgres-serials-should-be-bigint-and-how-to-migrate
- https://www.postgresql.org/docs/current/sql-altertable.html
- https://www.postgresql.org/docs/current/sql-insert.html
- https://www.postgresql.org/docs/release/18.0/
- https://www.rfc-editor.org/rfc/rfc9562
- https://www.postgresql.org/docs/current/ddl-constraints.html
- https://www.cybertec-postgresql.com/en/lookup-table-or-enum-type/
- https://github.com/ankane/strong_migrations
- https://www.cybertec-postgresql.com/en/entity-attribute-value-eav-design-in-postgresql-dont-do-it/
- https://www.crunchydata.com/blog/indexing-jsonb-in-postgres
- https://www.postgresql.org/docs/current/arrays.html
- https://www.postgresql.org/docs/current/ddl-generated-columns.html
- https://www.postgresql.org/docs/current/storage-toast.html
- https://docs.gitlab.com/development/database/ordering_table_columns/
