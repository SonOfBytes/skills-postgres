# Naming: Identifiers, Indexes and Constraints

Names are the part of the schema every tool, migration and colleague reads. Two rules do most
of the work: lowercase snake_case, and names derivable from the objects they belong to.

## Identifiers

- Lowercase `snake_case` for every table, column, index, constraint, function and schema.
  Postgres folds unquoted identifiers to lowercase, so a mixed-case name forces double quotes in
  every query forever and produces baffling "column does not exist" errors [wiki Don't Do This].
- Under 63 bytes. Postgres truncates silently at 63, and two long names that share a prefix
  collide without an error [GitLab constraint naming; squawk `identifier-too-long`].
- Tables singular or plural, never mixed; columns singular. The choice matters less than every
  table carrying the same one; pick the project's existing convention [GitLab naming].
- Timestamp columns end in `_at` (`created_at`, `claimed_at`), dates in `_on`, booleans read as
  predicates (`completed`, `allow_shorts`), foreign keys as `<referenced_singular>_id`.
- Reserved words (`user`, `order`, `group`, `key`) are legal but need quoting in some tools;
  prefer `users`, `orders`, `groups`, `api_key`.

## Indexes and constraints

Use a mechanical scheme so a name can be reconstructed from the table and columns. GitLab's is a
good default [GitLab constraint naming]:

| Object | Pattern | Example |
|---|---|---|
| Primary key | `pk_<table>` (or the default `<table>_pkey`) | `pk_channel_approvals` |
| Foreign key | `fk_<table>_<column>[_and_<column>]_<referenced_table>` | `fk_videos_channel_id_channels` |
| Index | `index_<table>_on_<column>[_and_<column>]` or `<table>_<purpose>_idx` | `index_videos_on_channel_id_and_published_at`, `videos_series_idx` |
| Unique | `unique_<table>_<column>[_and_<column>]` | `unique_users_subject` |
| Check | `check_<table>_<column>[_<suffix>]` | `check_tracks_kind_playlist_id` |
| Exclusion | `excl_<table>_<columns>_<suffix>` | `excl_reservations_room_id_during_no_overlap` |

When a name would exceed 63 bytes, drop the `_and_` joiners first, then abbreviate columns, in
that order, so truncation is predictable [GitLab constraint naming].

A purpose-named index (`videos_series_idx`, `package_jobs_claim_idx`) is preferable to a
column-list name when the index exists for one documented query; say which query in a comment
beside the migration.

## Schemas

- One database, one schema is the common case; separate schemas for module or tenant
  separation; separate databases only for unrelated applications (no cross-database joins)
  [Crunchy databases and schemas].
- Schema-qualify objects in migrations so what is created does not depend on the session's
  `search_path` [squawk `require-table-schema`]. Security consequences of `search_path` are in
  `security.md`.

## Migrations

- File names describe the change in the imperative: `000012_add_channel_pins`,
  `000013_index_videos_series`. One change class per file (see `migrations.md`).

## Comments

- `COMMENT ON COLUMN` for anything whose meaning is not obvious from the name and type: the unit
  of an interval, why a column is deliberately not unique, which revision a position was recorded
  against. Comments survive dumps and appear in every tool.

```sql
COMMENT ON COLUMN users.email IS
  'Descriptive only, refreshed from the identity token. Deliberately not unique.';
```

## Sources

- https://wiki.postgresql.org/wiki/Don%27t_Do_This
- https://docs.gitlab.com/development/database/constraint_naming_convention/
- https://squawkhq.com/docs/rules
- https://www.crunchydata.com/blog/postgres-databases-and-schemas
