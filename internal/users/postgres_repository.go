package users

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

func (r *PostgresRepository) Create(ctx context.Context, username, passwordHash string) (string, error) {
	var id string
	row := r.pool.QueryRow(ctx, `INSERT INTO users (username, password_hash) VALUES ($1, $2) RETURNING id`, username, passwordHash)
	if err := row.Scan(&id); err != nil {
		return "", err
	}
	return id, nil
}

func (r *PostgresRepository) GetByUsername(ctx context.Context, username string) (User, error) {
	var u User
	row := r.pool.QueryRow(ctx, `SELECT id, username, password_hash FROM users WHERE username = $1`, username)
	if err := row.Scan(&u.ID, &u.Username, &u.PasswordHash); err != nil {
		return User{}, err
	}
	return u, nil
}

func (r *PostgresRepository) Exists(ctx context.Context, id string) (bool, error) {
	var exists bool
	row := r.pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM users WHERE id = $1)`, id)
	if err := row.Scan(&exists); err != nil {
		return false, err
	}
	return exists, nil
}
