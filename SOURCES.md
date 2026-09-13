# Sources & Provenance

Every rule in `references/` cites a source, and every source here has recognised credibility.
The tiers are the acceptance policy; `verify/check_sources.py` reads the `## Allowed origins`
section and fails the build for any URL outside it, in either direction (a cited URL missing
here, or a listed URL nothing cites).

| Tier | Accepted as | Meaning |
|---|---|---|
| T1 | Authoritative | official PostgreSQL documentation, release notes and wiki; maintainer documentation for a named tool; standards bodies |
| T2 | Recognised vendor or handbook | the major Postgres companies' engineering material; large public engineering handbooks |
| T3 | Named contributor or practitioner | a core contributor or widely recognised practitioner, named on the origin line |

Not accepted: unknown personal blogs, content-marketing sites, Medium posts, third-party mirrors.
A claim that rests only on one is re-sourced or dropped.

## Allowed origins

T1 | postgresql.org | *
T1 | wiki.postgresql.org | *
T1 | pgbouncer.org | *
T1 | pgbackrest.org | *
T1 | pgtap.org | *
T1 | pkg.go.dev | /github.com/jackc/
T1 | github.com | /jackc/,/pgaudit/,/pgvector/,/citusdata/,/ankane/,/pressly/,/docker-library/,/prometheus-community/,/postgresml/,/golang-migrate/,/testcontainers/
T1 | docs.sqlc.dev | *
T1 | npgsql.org | *
T1 | learn.microsoft.com | /en-us/ef/,/en-us/dotnet/
T1 | squawkhq.com | *
T1 | atlasgo.io | *
T1 | xataio.github.io | /pgroll/
T1 | documentation.red-gate.com | /flyway/
T1 | golang.testcontainers.org | *
T1 | testcontainers.com | *
T1 | dotnet.testcontainers.org | *
T1 | docs.percona.com | /pg-tde/
T1 | cheatsheetseries.owasp.org | *
T1 | cisecurity.org | *
T1 | opentelemetry.io | /docs/specs/semconv/
T1 | kubernetes.io | /docs/
T1 | rfc-editor.org | *
T2 | crunchydata.com | *
T2 | cybertec-postgresql.com | *
T2 | percona.com | *
T2 | enterprisedb.com | *
T2 | pganalyze.com | *
T2 | postgres.ai | *
T2 | citusdata.com | *
T2 | supabase.com | /docs/
T2 | pgedge.com | *
T2 | docs.gitlab.com | *
T2 | postgres.fm | *
T3 | brandur.org | * (Brandur Leach)
T3 | depesz.com | * (Hubert "depesz" Lubaczewski)
T3 | hakibenita.com | * (Haki Benita)
T3 | thebuild.com | * (Christophe Pettus)
T3 | use-the-index-luke.com | * (Markus Winand)

## Sources used

The table is regenerated as references are written; `check_sources.py` keeps it exact. One line
per URL, what it contributed.

### T1 — PostgreSQL project

| URL | Used for |
|---|---|
