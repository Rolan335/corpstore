package pg

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// DB provides transactional operations to persist file metadata and user relations.
type PoolProvider struct {
	pool *pgxpool.Pool
}

func NewPoolPrv(ctx context.Context, dsn string) (*PoolProvider, error) {
	p, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, fmt.Errorf("connect db: %w", err)
	}

	return &PoolProvider{pool: p}, nil
}

func (d *PoolProvider) Tx(ctx context.Context, hdl Handler) error {
	tx, err := d.pool.Begin(ctx)
	if err != nil {
		return errors.Join(errPgxpoolBegin, err)
	}

	defer tx.Rollback(ctx)

	if err := hdl(tx); err != nil {
		return errors.Join(errPgxpoolExec, err)
	}

	if err := tx.Commit(ctx); err != nil {
		return errors.Join(errPgxpoolCommit, err)
	}

	return nil
}

func (d *PoolProvider) Close() {
	d.pool.Close()
}

func (d *PoolProvider) Ping(ctx context.Context) error {
	return d.pool.Ping(ctx)
}
