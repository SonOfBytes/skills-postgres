# Security: Roles, Authentication, Injection, Encryption, Logging

Calibrate to the deployment: a managed service owns its `pg_hba.conf` and disk encryption; a
personal tool does not need pgaudit. What never calibrates away: least privilege, bound
parameters, no secrets in the tree, nothing sensitive stored in plaintext. Row-level security
has its own file, `rls.md`.

## Roles and privileges

- The application role is **not** superuser and does **not** own its tables. Ownership sits with
  a migrator role; only owners and superusers can `DROP` or `ALTER`, so a compromised application
  credential cannot destroy the schema [Crunchy safer application users].
- Model privileges as `NOLOGIN` group roles and grant them to `LOGIN` roles that hold only
  credentials; rotating a credential then never re-derives a grant set [Crunchy public schema
  15].
- Grant the application `SELECT, INSERT, UPDATE` and, where the domain deletes, `DELETE`; never
  `TRUNCATE`, never DDL. Revoking `DELETE` in favour of a moved-row archive is a recommendation,
  not a rule (OWASP lists `DELETE` as a normal application grant) [Crunchy; OWASP].
- `ALTER DEFAULT PRIVILEGES FOR ROLE migrator IN SCHEMA app GRANT …` accompanies every role
  design: a plain `GRANT … ON ALL TABLES` covers only tables that exist today, and default
  privileges are scoped to the *creating* role, so without `FOR ROLE` they cover nothing the
  migrator creates [PG sql-alterdefaultprivileges].
- `[pg<=14]` and any database restored from a dump: `REVOKE CREATE ON SCHEMA public FROM
  PUBLIC` explicitly; 15 changed the default for new databases only [PG 15 release notes].
- Monitoring gets `pg_monitor`; backups get what they need; one account per application, removed
  when the application is decommissioned [postgres_exporter README; OWASP].

```sql
CREATE ROLE app_owner NOLOGIN;
CREATE ROLE app_rw NOLOGIN;
CREATE ROLE migrator LOGIN IN ROLE app_owner;
CREATE ROLE app LOGIN IN ROLE app_rw;

CREATE SCHEMA app AUTHORIZATION app_owner;
REVOKE CREATE ON SCHEMA public FROM PUBLIC;

GRANT USAGE ON SCHEMA app TO app_rw;
ALTER DEFAULT PRIVILEGES FOR ROLE app_owner IN SCHEMA app
  GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO app_rw;
ALTER DEFAULT PRIVILEGES FOR ROLE app_owner IN SCHEMA app
  GRANT USAGE ON SEQUENCES TO app_rw;

ALTER ROLE app SET search_path = app;
ALTER ROLE app SET statement_timeout = '15s';
```

## Authentication and transport

- `password_encryption = 'scram-sha-256'` and `scram-sha-256` on every `pg_hba.conf` line. `md5`
  is deprecated as of 18 and slated for removal; `trust` on a network line lets anyone be anyone
  [PG auth-password; PG 18 release notes].
- `pg_hba.conf` is first-match-wins with no fall-through: a permissive line above a strict one
  disables the strict one. Use `hostssl`, not `host`, for every remote record; `host` matches
  cleartext connections too [PG auth-pg-hba-conf].
- Application connection strings carry `sslmode=verify-full` and `sslrootcert`; `prefer` (the
  libpq default) and `require` do not authenticate the server. A Unix socket or a private
  container network may be documented as the exemption [PG libpq-ssl].
- Client certificates: `clientcert=verify-full` on the `hostssl` record, which binds the
  certificate's CN to the database user [PG auth-pg-hba-conf].
- `[pg>=18]` OAuth authentication for human access and `ssl_tls13_ciphers` for cipher policy
  [PG 18 release notes].
- Credentials come from a secret file or manager (`POSTGRES_PASSWORD_FILE` in containers), never
  a committed file, never argv, never a logged statement [official Docker image docs; PG
  runtime-config-logging].

## Injection and identifiers

- Every value is a bound parameter. There is no safe string interpolation of a value into SQL.
- Identifiers cannot be parameters; when a table or column name must be dynamic, use
  `format('%I', name)` or `quote_ident()`, and `%L` or `quote_nullable()` for literals (never
  `quote_literal()` on a nullable). Never dollar-quote a dynamic value [PG plpgsql-statements].
- `SECURITY DEFINER` functions pin their search path with `pg_temp` **last**, and are created in
  the same transaction that revokes `EXECUTE` from `PUBLIC`, because `EXECUTE` is public by
  default [PG sql-createfunction].

```sql
CREATE OR REPLACE FUNCTION app.claim_invite(p_email text)
RETURNS uuid
LANGUAGE sql
SECURITY DEFINER
SET search_path = app, pg_temp
AS $$
  UPDATE app.household_invites SET claimed_at = now()
  WHERE lower(email) = lower(p_email) AND claimed_at IS NULL
  RETURNING id;
$$;
REVOKE ALL ON FUNCTION app.claim_invite(text) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION app.claim_invite(text) TO app_rw;
```

- `search_path` hijacking (CVE-2018-1058): an attacker who can create objects in a schema on your
  path shadows `lower()` or a table name. Revoke `CREATE` on `public`, set `search_path` per role,
  and schema-qualify in functions and migrations [wiki CVE-2018-1058].
- Extensions are installed by the migrator, listed in the data-model document, and limited to
  what the schema uses; an extension is code running as the server [Crunchy CIS checklist].
- Driver error text never reaches a client response; map SQLSTATE to a domain error at the
  store boundary (`drivers/`) [OWASP].

## Encryption and secrets at rest

- At rest: volume or filesystem encryption, or a named extension (`pg_tde`). Transparent data
  encryption is not in community Postgres; patches have been proposed since 2016 and remain
  unmerged [wiki TDE; Percona pg_tde].
- Passwords hashed in the application (argon2id, scrypt, bcrypt); if in the database, `crypt()`
  with `gen_salt()` and an adaptive algorithm with a raised cost, never `digest()` [PG pgcrypto].
- Session tokens stored as a hash (SHA-256 of a high-entropy random token), never the token, so a
  dump does not hand over live sessions [OWASP].
- Column encryption with pgcrypto is server-side: keys cross the wire and can appear in logs;
  prefer client-side encryption for reversible secrets, and the `pgp_sym_*` family over
  `encrypt()`/`decrypt()` (no integrity, no proper KDF) [PG pgcrypto].
- Personal data has a documented retention and erasure path that works through the foreign-key
  graph (`modelling.md`) [OWASP].

## Logging

- `log_statement = all` never in production: statements carry parameter values and may carry
  passwords, and `log_parameter_max_length` defaults to unbounded. Use
  `log_min_duration_statement` with `log_min_duration_sample`, and clamp parameter length [PG
  runtime-config-logging].
- Application query tracers that log parameters run only at a level the configuration keeps off
  in production; OpenTelemetry marks `db.query.parameter.*` opt-in for the same reason [OTel db
  semconv].
- Compliance auditing uses pgaudit (session and object classes), scoped with `pgaudit.role`, with
  `pgaudit.log_parameter_max_size` bounded [pgaudit README].

## Baselines

The CIS PostgreSQL Benchmark (editions for 14 to 18, free for non-commercial use) is the
hardening checklist; the DISA STIG is the enforceable document in regulated environments; the
OWASP Database Security Cheat Sheet supplies the application-facing controls [CIS; Crunchy CIS
checklist; OWASP].

## Sources

- https://www.crunchydata.com/blog/safer-application-users-in-postgres
- https://www.crunchydata.com/blog/be-ready-public-schema-changes-in-postgres-15
- https://cheatsheetseries.owasp.org/cheatsheets/Database_Security_Cheat_Sheet.html
- https://www.postgresql.org/docs/current/sql-alterdefaultprivileges.html
- https://www.postgresql.org/docs/release/15.0/
- https://github.com/prometheus-community/postgres_exporter
- https://www.postgresql.org/docs/current/auth-password.html
- https://www.postgresql.org/docs/release/18.0/
- https://www.postgresql.org/docs/current/auth-pg-hba-conf.html
- https://www.postgresql.org/docs/current/libpq-ssl.html
- https://github.com/docker-library/docs/blob/master/postgres/README.md
- https://www.postgresql.org/docs/current/runtime-config-logging.html
- https://www.postgresql.org/docs/current/plpgsql-statements.html
- https://www.postgresql.org/docs/current/sql-createfunction.html
- https://wiki.postgresql.org/wiki/A_Guide_to_CVE-2018-1058:_Protect_Your_Search_Path
- https://www.crunchydata.com/blog/postgres-security-checklist-from-the-center-for-internet-security
- https://wiki.postgresql.org/wiki/Transparent_Data_Encryption
- https://docs.percona.com/pg-tde/
- https://www.postgresql.org/docs/current/pgcrypto.html
- https://opentelemetry.io/docs/specs/semconv/database/database-spans/
- https://github.com/pgaudit/pgaudit
- https://www.cisecurity.org/benchmark/postgresql
