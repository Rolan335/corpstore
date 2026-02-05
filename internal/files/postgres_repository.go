package files

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

func (r *PostgresRepository) Create(ctx context.Context, id, filename, ownerID string) error {
	_, err := r.pool.Exec(ctx, `INSERT INTO files (id, filename, owner_id) VALUES ($1, $2, $3)`, id, filename, ownerID)
	return err
}

func (r *PostgresRepository) GetByID(ctx context.Context, id string) (File, error) {
	var f File
	row := r.pool.QueryRow(ctx, `SELECT id, filename, owner_id, created_at FROM files WHERE id = $1`, id)
	if err := row.Scan(&f.ID, &f.Filename, &f.OwnerID, &f.CreatedAt); err != nil {
		return File{}, err
	}
	return f, nil
}

func (r *PostgresRepository) ListByOwner(ctx context.Context, ownerID string) ([]File, error) {
	rows, err := r.pool.Query(ctx, `SELECT id, filename, owner_id, created_at FROM files WHERE owner_id = $1 ORDER BY filename`, ownerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []File
	for rows.Next() {
		var f File
		if err := rows.Scan(&f.ID, &f.Filename, &f.OwnerID, &f.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	if rows.Err() != nil {
		return nil, rows.Err()
	}
	return out, nil
}
