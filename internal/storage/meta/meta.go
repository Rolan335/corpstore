package meta

import "context"

// MetaStore defines DB operations for file metadata and users.
type MetaStore interface {
	SaveFileMetadata(ctx context.Context, id string, filename string, ownerID string) error
	GetFileMetadata(ctx context.Context, id string) (string, string, error)
	CreateUser(ctx context.Context, username string) (string, error)
}
