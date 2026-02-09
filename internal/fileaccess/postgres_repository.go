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

func (r *PostgresRepository) Grant(ctx context.Context, fileID, userID string, ownerTG OwnerTGInfo) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO files_permissions (file_id, user_granted, owner_tg_first_name, owner_tg_last_name, owner_tg_username)
		VALUES ($1, $2, $3, $4, $5)
	`, fileID, userID, ownerTG.FirstName, ownerTG.LastName, ownerTG.Username)
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
		SELECT f.id, f.filename, f.owner_id, fp.owner_tg_first_name, fp.owner_tg_last_name, fp.owner_tg_username
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
		if err := rows.Scan(&f.ID, &f.Filename, &f.OwnerID, &f.OwnerTGFirstName, &f.OwnerTGLastName, &f.OwnerTGUsername); err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	if rows.Err() != nil {
		return nil, rows.Err()
	}
	return out, nil
}

func (r *PostgresRepository) ListGrantedUsers(ctx context.Context, fileID string) ([]GrantedUser, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT u.id, u.username
		FROM files_permissions fp
		JOIN users u ON u.id = fp.user_granted
		WHERE fp.file_id = $1
		ORDER BY u.username
	`, fileID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []GrantedUser
	for rows.Next() {
		var g GrantedUser
		if err := rows.Scan(&g.UserID, &g.Username); err != nil {
			return nil, err
		}
		out = append(out, g)
	}
	if rows.Err() != nil {
		return nil, rows.Err()
	}
	return out, nil
}

func (r *PostgresRepository) Revoke(ctx context.Context, fileID, userID string) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM files_permissions WHERE file_id = $1 AND user_granted = $2`, fileID, userID)
	return err
}

func (r *PostgresRepository) RevokeAllByFile(ctx context.Context, fileID string) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM files_permissions WHERE file_id = $1`, fileID)
	return err
}
