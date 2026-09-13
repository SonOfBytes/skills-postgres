# skills-postgres

A Claude Code agent skill for **PostgreSQL in application backends**: types and keys, data
modelling and tenancy, constraints, indexing, query patterns, transactions and locking, job
queues, full-text and vector search, partitioning, lock-safe migrations, roles and row-level
security, pooling, server settings, backups, observability, and data-layer testing.

Every rule cites a source of recognised credibility (official documentation, the PostgreSQL wiki,
maintainer documentation, standards bodies, the major Postgres vendors, or a named contributor);
`SOURCES.md` lists them by tier and `verify/check_sources.py` refuses anything else.

Covers Postgres **14 to 18** (19 in beta) with `[pg>=N]` / `[pg<=N]` markers rather than
per-version directories, because majors are additive. `SKILL.md` resolves the major version, the
deployment shape and the access layer first, then routes.

The review-side counterpart is the `postgres-quality-reviewer` agent, which shares these rules.

## Install

```
git clone git@github.com:SonOfBytes/skills-postgres.git ~/.claude/skills/postgres
```

The repo root is the skill directory, so the clone lands `SKILL.md` at the depth Claude Code scans.
`git pull` updates in place. Run `/reload-skills` (or start a new session) to pick it up.

## Layout

```
SKILL.md                    router: version, shape and access-layer detection + routing table
references/                 one file per topic, rules as "do X because Y [source]", worked SQL
references/drivers/         pgx, Npgsql/EF Core, sqlc
verify/pgx-seam/            real Go module — compiles, vets and tests against Postgres 16/17/18
verify/                     CI checks (frontmatter, links, SQL parses, sources allowed, version markers)
SOURCES.md                  every source, tiered; the machine-read allowed-origins list
```

## Verification

Every ```sql block in the references is parsed with `pglast` (libpg_query) on every push. Every
URL must appear in `SOURCES.md` and match an allowed origin. Every version marker must name a
major in `references/versions.md`. The Go seam module runs its tests against a live
`postgres:16`, `17` and `18` service in CI.

```
python3 -m venv .venv && .venv/bin/pip install -r verify/requirements.txt
.venv/bin/python verify/check_frontmatter.py
.venv/bin/python verify/check_links.py
.venv/bin/python verify/check_sqlblocks.py
.venv/bin/python verify/check_sources.py
.venv/bin/python verify/check_versions.py
```

## Related

[`skills-htmx-go`](https://github.com/SonOfBytes/skills-htmx-go) — the Go server side of htmx, the
sibling skill this one's layout follows.
