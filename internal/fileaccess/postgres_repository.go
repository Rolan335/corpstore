package fileaccess

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgresRepository implements Repository using pgxpool.
type PostgresRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

func (r *PostgresRepository) Grant(ctx context.Context, fileID, userID string) error {
	_, err := r.pool.Exec(ctx, `INSERT INTO files_permissions (file_id, user_granted) VALUES ($1, $2)`, fileID, userID)
	return err
}

func (r *PostgresRepository) HasAccess(ctx context.Context, fileID, userID string) (bool, error) {
	var exists bool
	row := r.pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM files_permissions WHERE file_id = $1 AND user_granted = $2)`, fileID, userID)
	if err := row.Scan(&exists); err != nil {
		return false, err
	}
	return exists, nil
}

func (r *PostgresRepository) ListSharedFiles(ctx context.Context, userID string) ([]SharedFile, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT f.id, f.filename, f.owner_id
		FROM files_permissions fp
		JOIN files f ON f.id = fp.file_id
		WHERE fp.user_granted = $1
		ORDER BY f.filename
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []SharedFile
	for rows.Next() {
		var f SharedFile
		if err := rows.Scan(&f.ID, &f.Filename, &f.OwnerID); err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	if rows.Err() != nil {
		return nil, rows.Err()
	}
	return out, nil
}
