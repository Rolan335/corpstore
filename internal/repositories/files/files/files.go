package files

import (
	"context"

	"github.com/jackc/pgx/v5"
)

type Repository struct {
}

func NewRepository() *Repository {
	return &Repository{}
}

// SaveFileMetadata saves file metadata (id, filename, owner) inside a transaction.
func (r *Repository) SaveFileMetadata(ctx context.Context, tx pgx.Tx, id string, filename string, ownerID string) error {

	_, err := tx.Exec(ctx, `INSERT INTO files (id, filename, owner_id) VALUES ($1, $2, $3)`, id, filename, ownerID)
	if err != nil {
		return err
	}

	return nil
}

// // GetFileMetadata returns filename and owner_id for a given file id
// func (d *DB) GetFileMetadata(ctx context.Context, id string) (string, string, error) {
// 	conn, err := d.pool.Acquire(ctx)
// 	if err != nil {
// 		return "", "", err
// 	}
// 	defer conn.Release()

// 	var filename string
// 	var ownerID string
// 	row := conn.QueryRow(ctx, `SELECT filename, owner_id FROM files WHERE id = $1`, id)
// 	if err := row.Scan(&filename, &ownerID); err != nil {
// 		return "", "", err
// 	}
// 	return filename, ownerID, nil
// }

// // ListFilesByOwner returns files owned by the given user.
// func (d *DB) ListFilesByOwner(ctx context.Context, ownerID string) ([]meta.FileInfo, error) {
// 	conn, err := d.pool.Acquire(ctx)
// 	if err != nil {
// 		return nil, err
// 	}
// 	defer conn.Release()

// 	rows, err := conn.Query(ctx, `SELECT id, filename, owner_id FROM files WHERE owner_id = $1 ORDER BY filename`, ownerID)
// 	if err != nil {
// 		return nil, err
// 	}
// 	defer rows.Close()

// 	var out []meta.FileInfo
// 	for rows.Next() {
// 		var fi meta.FileInfo
// 		if err := rows.Scan(&fi.ID, &fi.Filename, &fi.OwnerID); err != nil {
// 			return nil, err
// 		}
// 		out = append(out, fi)
// 	}
// 	if rows.Err() != nil {
// 		return nil, rows.Err()
// 	}
// 	return out, nil
// }
