package seam

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const schema = `
DROP TABLE IF EXISTS seam_videos, seam_jobs;
DROP ROLE IF EXISTS seam_app;
CREATE ROLE seam_app LOGIN PASSWORD 'seam_app';
CREATE TABLE seam_videos (
  id           bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  tenant_id    bigint NOT NULL,
  title        text NOT NULL CHECK (length(title) BETWEEN 1 AND 200),
  published_at timestamptz NOT NULL,
  UNIQUE (tenant_id, title)
);
CREATE INDEX seam_videos_series_idx ON seam_videos (tenant_id, published_at, id);
ALTER TABLE seam_videos ENABLE ROW LEVEL SECURITY;
ALTER TABLE seam_videos FORCE ROW LEVEL SECURITY;
CREATE POLICY seam_videos_tenant ON seam_videos TO seam_app
  USING (tenant_id = (SELECT NULLIF(current_setting('app.tenant_id', true), '')::bigint))
  WITH CHECK (tenant_id = (SELECT NULLIF(current_setting('app.tenant_id', true), '')::bigint));
GRANT SELECT, INSERT, UPDATE, DELETE ON seam_videos TO seam_app;
CREATE TABLE seam_jobs (
  id        bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  priority  smallint NOT NULL DEFAULT 50,
  state     text NOT NULL DEFAULT 'queued' CHECK (state IN ('queued','leased','done')),
  leased_by text,
  attempts  int NOT NULL DEFAULT 0
);
CREATE INDEX seam_jobs_claim_idx ON seam_jobs (priority, id) WHERE state = 'queued';
`

var (
	pool    *pgxpool.Pool
	appPool *pgxpool.Pool
)

func TestMain(m *testing.M) {
	dsn := os.Getenv("PGX_SEAM_DATABASE_URL")
	if dsn == "" {
		os.Stderr.WriteString("PGX_SEAM_DATABASE_URL not set: the seam tests need a throwaway Postgres\n")
		os.Exit(1)
	}
	ctx := context.Background()
	var err error
	pool, err = NewPool(ctx, dsn)
	if err != nil {
		panic(err)
	}
	if _, err := pool.Exec(ctx, schema); err != nil {
		panic(err)
	}
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		panic(err)
	}
	cfg.ConnConfig.User, cfg.ConnConfig.Password = "seam_app", "seam_app"
	cfg.ConnConfig.DefaultQueryExecMode = pgx.QueryExecModeExec
	appPool, err = pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		panic(err)
	}
	code := m.Run()
	pool.Close()
	appPool.Close()
	os.Exit(code)
}

func reset(t *testing.T) {
	t.Helper()
	if _, err := pool.Exec(context.Background(), `TRUNCATE seam_videos, seam_jobs RESTART IDENTITY`); err != nil {
		t.Fatal(err)
	}
}

func seed(t *testing.T, tenant int64, titles ...string) {
	t.Helper()
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	for i, title := range titles {
		_, err := pool.Exec(context.Background(),
			`INSERT INTO seam_videos (tenant_id, title, published_at) VALUES ($1, $2, $3)`,
			tenant, title, base.Add(time.Duration(i/2)*time.Hour)) // pairs share a timestamp: the tie case
		if err != nil {
			t.Fatal(err)
		}
	}
}

func TestMapErrorTranslatesSQLSTATE(t *testing.T) {
	reset(t)
	ctx := context.Background()
	seed(t, 1, "a")
	_, err := pool.Exec(ctx, `INSERT INTO seam_videos (tenant_id, title, published_at) VALUES (1, 'a', now())`)
	if !errors.Is(MapError(err), ErrAlreadyExists) {
		t.Fatalf("unique violation mapped to %v", MapError(err))
	}
	_, err = pool.Exec(ctx, `INSERT INTO seam_videos (tenant_id, title, published_at) VALUES (1, '', now())`)
	if !errors.Is(MapError(err), ErrInvalidInput) {
		t.Fatalf("check violation mapped to %v", MapError(err))
	}
	err = pool.QueryRow(ctx, `SELECT id FROM seam_videos WHERE id = -1`).Scan(new(int64))
	if !errors.Is(MapError(err), ErrNotFound) {
		t.Fatalf("no rows mapped to %v", MapError(err))
	}
}

func TestKeysetPageHandlesTies(t *testing.T) {
	reset(t)
	seed(t, 1, "a", "b", "c", "d", "e")
	ctx := context.Background()
	var got []string
	var after *Video
	for {
		page, err := Page(ctx, pool, 1, after, 2)
		if err != nil {
			t.Fatal(err)
		}
		if len(page) == 0 {
			break
		}
		for _, v := range page {
			got = append(got, v.Title)
		}
		after = &page[len(page)-1]
	}
	want := []string{"a", "b", "c", "d", "e"}
	if len(got) != len(want) {
		t.Fatalf("walked %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("walked %v, want %v", got, want)
		}
	}
}

func TestSkipLockedClaimsAreDisjoint(t *testing.T) {
	reset(t)
	ctx := context.Background()
	for range 4 {
		if _, err := pool.Exec(ctx, `INSERT INTO seam_jobs DEFAULT VALUES`); err != nil {
			t.Fatal(err)
		}
	}
	txA, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer txA.Rollback(ctx)
	txB, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer txB.Rollback(ctx)

	a, err := Claim(ctx, txA, "A", 2)
	if err != nil {
		t.Fatal(err)
	}
	b, err := Claim(ctx, txB, "B", 2) // must not block on A's rows, and must not receive them
	if err != nil {
		t.Fatal(err)
	}
	if len(a) != 2 || len(b) != 2 {
		t.Fatalf("A=%v B=%v, want two each", a, b)
	}
	for _, x := range a {
		for _, y := range b {
			if x == y {
				t.Fatalf("job %d handed to both workers", x)
			}
		}
	}
}

func TestRLSIsolatesTenantsForTheAppRole(t *testing.T) {
	reset(t)
	seed(t, 1, "one")
	seed(t, 2, "two")
	ctx := context.Background()

	var titles []string
	err := WithTenant(ctx, appPool, 1, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `SELECT title FROM seam_videos ORDER BY title`)
		if err != nil {
			return err
		}
		titles, err = pgx.CollectRows(rows, pgx.RowTo[string])
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(titles) != 1 || titles[0] != "one" {
		t.Fatalf("tenant 1 sees %v, want [one]", titles)
	}

	// The next checkout of the same pool must not inherit the tenant: SET LOCAL died with the transaction.
	var n int
	if err := appPool.QueryRow(ctx, `SELECT count(*) FROM seam_videos`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Fatalf("unscoped app-role query saw %d rows, want 0 (tenant leaked or policy missing)", n)
	}

	// The owner bypasses nothing here because FORCE is set: the owner pool still sees both without a tenant.
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM seam_videos`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Fatalf("superuser pool saw %d rows, want 2", n)
	}
}
