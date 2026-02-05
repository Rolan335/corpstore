package files

import "context"

// Repository defines files table operations.
type Repository interface {
	Create(ctx context.Context, id, filename, ownerID string) error
	GetByID(ctx context.Context, id string) (File, error)
	ListByOwner(ctx context.Context, ownerID string) ([]File, error)
}
