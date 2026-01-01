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

func (d *DB) Ping(ctx context.Context) error {
	return d.pool.Ping(ctx)
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
func (d *DB) CreateUser(ctx context.Context, username string, passwordHash string) (string, error) {
	conn, err := d.pool.Acquire(ctx)
	if err != nil {
		return "", err
	}
	defer conn.Release()

	var id string
	row := conn.QueryRow(ctx, `INSERT INTO users (username, password_hash) VALUES ($1, $2) RETURNING id`, username, passwordHash)
	if err := row.Scan(&id); err != nil {
		return "", err
	}
	return id, nil
}

// GetUserByUsername returns id and password hash for username
func (d *DB) GetUserByUsername(ctx context.Context, username string) (string, string, error) {
	conn, err := d.pool.Acquire(ctx)
	if err != nil {
		return "", "", err
	}
	defer conn.Release()

	var id string
	var passwordHash string
	row := conn.QueryRow(ctx, `SELECT id, password_hash FROM users WHERE username = $1`, username)
	if err := row.Scan(&id, &passwordHash); err != nil {
		return "", "", err
	}
	return id, passwordHash, nil
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

// UserExists reports whether a user with the given id exists.
func (d *DB) UserExists(ctx context.Context, id string) (bool, error) {
	conn, err := d.pool.Acquire(ctx)
	if err != nil {
		return false, err
	}
	defer conn.Release()

	var exists bool
	row := conn.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM users WHERE id = $1)`, id)
	if err := row.Scan(&exists); err != nil {
		return false, err
	}
	return exists, nil
}

// ListFilesByOwner returns files owned by the given user.
func (d *DB) ListFilesByOwner(ctx context.Context, ownerID string) ([]meta.FileInfo, error) {
	conn, err := d.pool.Acquire(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Release()

	rows, err := conn.Query(ctx, `SELECT id, filename, owner_id FROM files WHERE owner_id = $1 ORDER BY filename`, ownerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []meta.FileInfo
	for rows.Next() {
		var fi meta.FileInfo
		if err := rows.Scan(&fi.ID, &fi.Filename, &fi.OwnerID); err != nil {
			return nil, err
		}
		out = append(out, fi)
	}
	if rows.Err() != nil {
		return nil, rows.Err()
	}
	return out, nil
}