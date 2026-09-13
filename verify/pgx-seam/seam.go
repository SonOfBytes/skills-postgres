// Package seam holds the pgx patterns documented in references/drivers/pgx.md,
// compiled and tested against a real Postgres so the reference cannot drift
// from what the driver actually does.
package seam

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Domain errors the store boundary maps SQLSTATEs to.
var (
	ErrAlreadyExists    = errors.New("already exists")
	ErrInvalidReference = errors.New("invalid reference")
	ErrInvalidInput     = errors.New("invalid input")
	ErrNotFound         = errors.New("not found")
	ErrRetryable        = errors.New("retryable")
)

// NewPool opens a pool with every default the reference says to override.
func NewPool(ctx context.Context, dsn string) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("parse database url: %w", err)
	}
	cfg.MaxConns = 10
	cfg.MinConns = 2
	cfg.MaxConnLifetime = time.Hour
	cfg.MaxConnLifetimeJitter = 10 * time.Minute
	cfg.MaxConnIdleTime = 15 * time.Minute
	cfg.HealthCheckPeriod = time.Minute
	cfg.ConnConfig.ConnectTimeout = 5 * time.Second
	cfg.ConnConfig.RuntimeParams["application_name"] = "pgx-seam"
	cfg.ConnConfig.RuntimeParams["TimeZone"] = "UTC"
	// Safe behind a transaction pooler; see references/drivers/pgx.md.
	cfg.ConnConfig.DefaultQueryExecMode = pgx.QueryExecModeExec
	return pgxpool.NewWithConfig(ctx, cfg)
}

// MapError translates a driver error into the domain vocabulary.
func MapError(err error) error {
	if err == nil {
		return nil
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case pgerrcode.UniqueViolation:
			return fmt.Errorf("%w: %s", ErrAlreadyExists, pgErr.ConstraintName)
		case pgerrcode.ForeignKeyViolation:
			return fmt.Errorf("%w: %s", ErrInvalidReference, pgErr.ConstraintName)
		case pgerrcode.CheckViolation:
			return fmt.Errorf("%w: %s", ErrInvalidInput, pgErr.ConstraintName)
		case pgerrcode.SerializationFailure, pgerrcode.DeadlockDetected:
			return fmt.Errorf("%w: %s", ErrRetryable, pgErr.Code)
		}
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	return err
}

// Video is the row shape scanned by name, so column order cannot break it.
type Video struct {
	ID          int64     `db:"id"`
	TenantID    int64     `db:"tenant_id"`
	Title       string    `db:"title"`
	PublishedAt time.Time `db:"published_at"`
}

// Page returns one keyset page for a tenant, ordered by (published_at, id).
func Page(ctx context.Context, pool *pgxpool.Pool, tenantID int64, after *Video, limit int) ([]Video, error) {
	var afterAt *time.Time
	var afterID *int64
	if after != nil {
		afterAt, afterID = &after.PublishedAt, &after.ID
	}
	rows, err := pool.Query(ctx, `
		SELECT id, tenant_id, title, published_at
		FROM seam_videos
		WHERE tenant_id = $1
		  AND ($2::timestamptz IS NULL OR (published_at, id) > ($2, $3::bigint))
		ORDER BY published_at ASC, id ASC
		LIMIT $4`, tenantID, afterAt, afterID, limit)
	if err != nil {
		return nil, fmt.Errorf("page: %w", err)
	}
	return pgx.CollectRows(rows, pgx.RowToStructByName[Video])
}

// Claim leases up to n queued jobs with SKIP LOCKED inside the caller's transaction.
func Claim(ctx context.Context, tx pgx.Tx, worker string, n int) ([]int64, error) {
	rows, err := tx.Query(ctx, `
		UPDATE seam_jobs SET state = 'leased', leased_by = $1, attempts = attempts + 1
		WHERE id IN (
			SELECT id FROM seam_jobs
			WHERE state = 'queued'
			ORDER BY priority ASC, id ASC
			LIMIT $2
			FOR UPDATE SKIP LOCKED
		)
		RETURNING id`, worker, n)
	if err != nil {
		return nil, fmt.Errorf("claim: %w", err)
	}
	return pgx.CollectRows(rows, pgx.RowTo[int64])
}

// WithTenant runs fn inside a transaction whose tenant GUC is set with SET LOCAL,
// so the value dies with the transaction and never leaks to the next checkout.
func WithTenant(ctx context.Context, pool *pgxpool.Pool, tenantID int64, fn func(pgx.Tx) error) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck // no-op after Commit

	// SET LOCAL cannot take a bind parameter; the value is formatted as a literal
	// through the server's own quoting.
	if _, err := tx.Exec(ctx, `SELECT set_config('app.tenant_id', $1, true)`, fmt.Sprint(tenantID)); err != nil {
		return fmt.Errorf("set tenant: %w", err)
	}
	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
