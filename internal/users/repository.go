package users

import "context"

// Repository defines users table operations.
type Repository interface {
	Create(ctx context.Context, username, passwordHash string) (string, error)
	GetByUsername(ctx context.Context, username string) (User, error)
	Exists(ctx context.Context, id string) (bool, error)
}
