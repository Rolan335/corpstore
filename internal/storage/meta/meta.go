package meta

import "context"

// MetaStore defines DB operations for file metadata and users.
type Store interface {
	SaveFileMetadata(ctx context.Context, id string, filename string, ownerID string) error
	GetFileMetadata(ctx context.Context, id string) (string, string, error)
	// CreateUser creates a user with optional password hash and returns user id
	CreateUser(ctx context.Context, username string, passwordHash string) (string, error)
	// GetUserByUsername returns id and password hash for username
	GetUserByUsername(ctx context.Context, username string) (string, string, error)
	// UserExists reports whether a user with the given id exists.
	UserExists(ctx context.Context, id string) (bool, error)
	// ListFilesByOwner returns files owned by the given user.
	ListFilesByOwner(ctx context.Context, ownerID string) ([]FileInfo, error)
}

// FileInfo represents a stored file metadata record.
type FileInfo struct {
	ID       string `json:"id"`
	Filename string `json:"filename"`
	OwnerID  string `json:"owner_id"`
}
