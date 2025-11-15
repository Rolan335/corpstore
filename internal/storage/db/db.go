package db

import (
	"context"
	"fmt"

	"corpstore/internal/storage/meta"

	"github.com/jackc/pgx/v5/pgxpool"
)

// DB provides transactional operations to persist file metadata and user relations.
type DB struct {
	pool *pgxpool.Pool
}

func NewDB(ctx context.Context, dsn string) (*DB, error) {
	p, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, fmt.Errorf("connect db: %w", err)
	}
	return &DB{pool: p}, nil
}

func (d *DB) Close() {
	d.pool.Close()
}

// SaveFileMetadata saves file metadata (id, filename, owner) inside a transaction.
func (d *DB) SaveFileMetadata(ctx context.Context, id string, filename string, ownerID string) error {
	conn, err := d.pool.Acquire(ctx)
	if err != nil {
		return err
	}
	defer conn.Release()

	tx, err := conn.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	_, err = tx.Exec(ctx, `INSERT INTO files (id, filename, owner_id) VALUES ($1, $2, $3)`, id, filename, ownerID)
	if err != nil {
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return err
	}
	return nil
}

// CreateUser creates a user and returns its id
func (d *DB) CreateUser(ctx context.Context, username string) (string, error) {
	conn, err := d.pool.Acquire(ctx)
	if err != nil {
		return "", err
	}
	defer conn.Release()

	var id string
	row := conn.QueryRow(ctx, `INSERT INTO users (username) VALUES ($1) RETURNING id`, username)
	if err := row.Scan(&id); err != nil {
		return "", err
	}
	return id, nil
}

// GetFileMetadata returns filename and owner_id for a given file id
func (d *DB) GetFileMetadata(ctx context.Context, id string) (string, string, error) {
	conn, err := d.pool.Acquire(ctx)
	if err != nil {
		return "", "", err
	}
	defer conn.Release()

	var filename string
	var ownerID string
	row := conn.QueryRow(ctx, `SELECT filename, owner_id FROM files WHERE id = $1`, id)
	if err := row.Scan(&filename, &ownerID); err != nil {
		return "", "", err
	}
	return filename, ownerID, nil
}

// Ensure DB implements meta.MetaStore
var _ meta.MetaStore = (*DB)(nil)
