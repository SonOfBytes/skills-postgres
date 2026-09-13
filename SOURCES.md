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

One line per URL and the reference files that cite it. `verify/check_sources.py` keeps this
exact in both directions.

### T1 — Authoritative

| URL | Cited by |
|---|---|
| https://atlasgo.io/concepts/declarative-vs-versioned | migrations |
| https://atlasgo.io/lint/analyzers | constraints, migration-recipes, migrations |
| https://cheatsheetseries.owasp.org/cheatsheets/Database_Security_Cheat_Sheet.html | modelling, operations, security |
| https://docs.percona.com/pg-tde/ | security |
| https://docs.sqlc.dev/en/stable/howto/select.html | drivers/sqlc, queries |
| https://docs.sqlc.dev/en/stable/howto/vet.html | drivers/sqlc |
| https://docs.sqlc.dev/en/stable/reference/config.html | drivers/sqlc |
| https://documentation.red-gate.com/flyway/reference/commands/validate | migrations |
| https://dotnet.testcontainers.org/modules/postgres/ | drivers/npgsql |
| https://github.com/ankane/strong_migrations | indexing, migration-recipes, migrations, types |
| https://github.com/citusdata/pg_cron | capabilities |
| https://github.com/docker-library/docs/blob/master/postgres/README.md | backup-and-observability, operations, security |
| https://github.com/jackc/pgx/wiki/Getting-started-with-pgx | drivers/pgx |
| https://github.com/pgaudit/pgaudit | security |
| https://github.com/pgvector/pgvector | capabilities |
| https://github.com/postgresml/pgcat | pooling |
| https://github.com/pressly/goose | migrations |
| https://github.com/prometheus-community/postgres_exporter | backup-and-observability, security |
| https://golang.testcontainers.org/modules/postgres/ | drivers/pgx, testing |
| https://kubernetes.io/docs/concepts/workloads/pods/probes/ | backup-and-observability |
| https://learn.microsoft.com/en-us/ef/core/managing-schemas/migrations/ | drivers/npgsql |
| https://learn.microsoft.com/en-us/ef/core/performance/efficient-querying | drivers/npgsql, queries |
| https://opentelemetry.io/docs/specs/semconv/database/database-spans/ | backup-and-observability, security |
| https://pgbackrest.org/user-guide.html | backup-and-observability |
| https://pgtap.org/ | testing |
| https://pkg.go.dev/github.com/jackc/pgx/v5 | drivers/pgx, pooling |
| https://pkg.go.dev/github.com/jackc/pgx/v5/pgconn | drivers/pgx |
| https://pkg.go.dev/github.com/jackc/pgx/v5/pgxpool | drivers/pgx, pooling |
| https://squawkhq.com/docs/rules | constraints, migration-recipes, migrations, naming, partitioning |
| https://testcontainers.com/guides/getting-started-with-testcontainers-for-go/ | testing |
| https://wiki.postgresql.org/wiki/A_Guide_to_CVE-2018-1058:_Protect_Your_Search_Path | security |
| https://wiki.postgresql.org/wiki/Don%27t_Do_This | naming, partitioning, queries, types |
| https://wiki.postgresql.org/wiki/Number_Of_Database_Connections | operations, pooling |
| https://wiki.postgresql.org/wiki/Slow_Query_Questions | queries |
| https://wiki.postgresql.org/wiki/Transparent_Data_Encryption | security |
| https://www.cisecurity.org/benchmark/postgresql | security |
| https://www.npgsql.org/doc/connection-string-parameters.html | drivers/npgsql, pooling |
| https://www.npgsql.org/doc/performance.html | drivers/npgsql |
| https://www.pgbouncer.org/config.html | pooling |
| https://www.pgbouncer.org/features.html | drivers/npgsql, pooling, transactions |
| https://www.postgresql.org/about/news/postgresql-18-released-3142/ | versions |
| https://www.postgresql.org/docs/current/arrays.html | types |
| https://www.postgresql.org/docs/current/auth-password.html | security |
| https://www.postgresql.org/docs/current/auth-pg-hba-conf.html | security |
| https://www.postgresql.org/docs/current/auto-explain.html | backup-and-observability |
| https://www.postgresql.org/docs/current/backup-dump.html | backup-and-observability |
| https://www.postgresql.org/docs/current/datatype-json.html | indexing |
| https://www.postgresql.org/docs/current/ddl-constraints.html | constraints, modelling, types |
| https://www.postgresql.org/docs/current/ddl-generated-columns.html | migration-recipes, types |
| https://www.postgresql.org/docs/current/ddl-partitioning.html | partitioning |
| https://www.postgresql.org/docs/current/ddl-rowsecurity.html | rls, testing |
| https://www.postgresql.org/docs/current/errcodes-appendix.html | transactions |
| https://www.postgresql.org/docs/current/explicit-locking.html | migrations, queues, testing, transactions |
| https://www.postgresql.org/docs/current/functions-admin.html | queues, transactions |
| https://www.postgresql.org/docs/current/functions-aggregate.html | queries |
| https://www.postgresql.org/docs/current/functions-comparisons.html | queries |
| https://www.postgresql.org/docs/current/functions-subquery.html | queries |
| https://www.postgresql.org/docs/current/hot-standby.html | transactions |
| https://www.postgresql.org/docs/current/indexes-index-only-scans.html | indexing |
| https://www.postgresql.org/docs/current/indexes-multicolumn.html | indexing |
| https://www.postgresql.org/docs/current/indexes-partial.html | indexing |
| https://www.postgresql.org/docs/current/indexes-types.html | indexing |
| https://www.postgresql.org/docs/current/jit-decision.html | operations |
| https://www.postgresql.org/docs/current/libpq-ssl.html | security |
| https://www.postgresql.org/docs/current/logicaldecoding.html | capabilities |
| https://www.postgresql.org/docs/current/monitoring-stats.html | backup-and-observability, indexing, operations |
| https://www.postgresql.org/docs/current/pgcrypto.html | security |
| https://www.postgresql.org/docs/current/pgstatstatements.html | backup-and-observability, operations |
| https://www.postgresql.org/docs/current/pgstattuple.html | operations |
| https://www.postgresql.org/docs/current/pgtrgm.html | capabilities, indexing |
| https://www.postgresql.org/docs/current/pgupgrade.html | backup-and-observability, versions |
| https://www.postgresql.org/docs/current/planner-stats.html | queries |
| https://www.postgresql.org/docs/current/plpgsql-statements.html | security |
| https://www.postgresql.org/docs/current/populate.html | queries |
| https://www.postgresql.org/docs/current/queries-limit.html | queries |
| https://www.postgresql.org/docs/current/queries-with.html | queries |
| https://www.postgresql.org/docs/current/rangetypes.html | capabilities, constraints |
| https://www.postgresql.org/docs/current/routine-vacuuming.html | backup-and-observability, operations, partitioning, queries |
| https://www.postgresql.org/docs/current/runtime-config-client.html | operations |
| https://www.postgresql.org/docs/current/runtime-config-logging.html | backup-and-observability, operations, security |
| https://www.postgresql.org/docs/current/runtime-config-query.html | operations, queries |
| https://www.postgresql.org/docs/current/runtime-config-replication.html | backup-and-observability, capabilities, operations |
| https://www.postgresql.org/docs/current/sql-alterdefaultprivileges.html | security |
| https://www.postgresql.org/docs/current/sql-altertable.html | constraints, migration-recipes, migrations, partitioning, types |
| https://www.postgresql.org/docs/current/sql-createfunction.html | security |
| https://www.postgresql.org/docs/current/sql-createindex.html | capabilities, indexing, migration-recipes, migrations |
| https://www.postgresql.org/docs/current/sql-insert.html | queries, types |
| https://www.postgresql.org/docs/current/sql-merge.html | queries |
| https://www.postgresql.org/docs/current/sql-notify.html | testing, transactions |
| https://www.postgresql.org/docs/current/sql-prepare.html | queries |
| https://www.postgresql.org/docs/current/sql-refreshmaterializedview.html | capabilities |
| https://www.postgresql.org/docs/current/sql-reindex.html | capabilities, indexing, migration-recipes, operations |
| https://www.postgresql.org/docs/current/sql-select.html | queries |
| https://www.postgresql.org/docs/current/sql-set.html | transactions |
| https://www.postgresql.org/docs/current/storage-hot.html | indexing, transactions |
| https://www.postgresql.org/docs/current/storage-toast.html | queries, types |
| https://www.postgresql.org/docs/current/textsearch-tables.html | capabilities |
| https://www.postgresql.org/docs/current/transaction-iso.html | testing, transactions |
| https://www.postgresql.org/docs/release/15.0/ | security, versions |
| https://www.postgresql.org/docs/release/16.0/ | versions |
| https://www.postgresql.org/docs/release/17.0/ | indexing, queries, versions |
| https://www.postgresql.org/docs/release/18.0/ | capabilities, constraints, indexing, migration-recipes, modelling, operations, partitioning, queries, security, types, versions |
| https://www.postgresql.org/support/versioning/ | versions |
| https://www.rfc-editor.org/rfc/rfc9562 | types |
| https://xataio.github.io/pgroll/ | migrations |

### T2 — Recognised vendors and handbooks

| URL | Cited by |
|---|---|
| https://docs.gitlab.com/development/database/constraint_naming_convention/ | naming |
| https://docs.gitlab.com/development/database/foreign_keys/ | constraints, migration-recipes |
| https://docs.gitlab.com/development/database/not_null_constraints/ | migration-recipes |
| https://docs.gitlab.com/development/database/ordering_table_columns/ | types |
| https://docs.gitlab.com/development/database/partitioning/ | partitioning |
| https://docs.gitlab.com/development/database/polymorphic_associations.html | modelling |
| https://docs.gitlab.com/development/migration_style_guide/ | migration-recipes, migrations |
| https://pganalyze.com/blog/deconstructing-the-postgres-planner | indexing |
| https://pganalyze.com/blog/how-postgres-chooses-index | indexing |
| https://pganalyze.com/blog/migrating-from-oracle-hints-to-pg-hint-plan-on-postgresql | modelling |
| https://pganalyze.com/blog/postgres-create-index | constraints |
| https://pganalyze.com/docs/explain/basics-of-postgres-query-planning | indexing, queries, testing |
| https://postgres.ai/blog/20210923-zero-downtime-postgres-schema-migrations-lock-timeout-and-retries | migrations, testing |
| https://postgres.fm/episodes/soft-delete | modelling |
| https://supabase.com/docs/guides/database/postgres/row-level-security | rls |
| https://supabase.com/docs/guides/database/tables | constraints |
| https://supabase.com/docs/guides/troubleshooting/rls-performance-and-best-practices-Z5Jjwv | modelling, rls |
| https://www.citusdata.com/blog/2023/07/31/schema-based-sharding-comes-to-postgres-with-citus/ | modelling |
| https://www.crunchydata.com/blog/be-ready-public-schema-changes-in-postgres-15 | security |
| https://www.crunchydata.com/blog/designing-your-postgres-database-for-multi-tenancy | modelling |
| https://www.crunchydata.com/blog/indexing-jsonb-in-postgres | types |
| https://www.crunchydata.com/blog/postgres-19-how-our-advice-has-changed-since-we-wrote-it | indexing, operations, partitioning, versions |
| https://www.crunchydata.com/blog/postgres-databases-and-schemas | naming |
| https://www.crunchydata.com/blog/postgres-indexes-for-newbies | indexing |
| https://www.crunchydata.com/blog/postgres-security-checklist-from-the-center-for-internet-security | capabilities, security |
| https://www.crunchydata.com/blog/postgres-serials-should-be-bigint-and-how-to-migrate | migration-recipes, operations, types |
| https://www.crunchydata.com/blog/row-level-security-for-tenants-in-postgres | rls |
| https://www.crunchydata.com/blog/safer-application-users-in-postgres | security |
| https://www.crunchydata.com/developers/playground/working-with-money-in-postgres | types |
| https://www.cybertec-postgresql.com/en/entity-attribute-value-eav-design-in-postgresql-dont-do-it/ | modelling, types |
| https://www.cybertec-postgresql.com/en/get-rid-of-your-unused-indexes/ | indexing |
| https://www.cybertec-postgresql.com/en/how-to-interpret-postgresql-explain-analyze-output/ | queries |
| https://www.cybertec-postgresql.com/en/index-your-foreign-key/ | constraints, indexing |
| https://www.cybertec-postgresql.com/en/lookup-table-or-enum-type/ | types |
| https://www.cybertec-postgresql.com/en/tuning-autovacuum-postgresql/ | operations |
| https://www.enterprisedb.com/postgres-tutorials/how-tune-postgresql-memory | operations |
| https://www.percona.com/blog/partitioning-free-lunches-and-indexing/ | partitioning |
| https://www.percona.com/blog/postgresql-partitioning-made-easy-using-pg_partman-timebased/ | partitioning |
| https://www.percona.com/blog/postgresql-partitioning-using-traditional-methods/ | partitioning |
| https://www.pgedge.com/blog/what-is-a-collation-and-why-is-my-data-corrupt | operations |

### T3 — Named practitioners

| URL | Cited by |
|---|---|
| https://brandur.org/idempotency-keys | constraints, modelling, transactions |
| https://brandur.org/job-drain | queues |
| https://brandur.org/postgres-queues | queues |
| https://brandur.org/soft-deletion | modelling |
| https://hakibenita.com/postgresql-unused-index-size | indexing |
| https://thebuild.com/blog/2024/11/15/the-doom-that-came-to-postgresql-when-collations-change/ | operations |
| https://use-the-index-luke.com/sql/where-clause/obfuscation | queries |
| https://www.depesz.com/2021/11/19/does-varcharn-use-less-disk-space-than-varchar-or-text/ | types |
