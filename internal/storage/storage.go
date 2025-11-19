package storage

import (
	"context"
)

// FileStore stores raw bytes and returns a generated UUID id
type FileStore interface {
	Save(ctx context.Context, data []byte) (string, error)
	Get(ctx context.Context, id string) ([]byte, error)
	Delete(ctx context.Context, id string) error
}

// Storage defines operations for saving and retrieving files (composed layer).
// Implementations combine FileStore and MetaStore for metadata.
type Storage interface {
	// Save persists file data and returns a generated id (UUID) for later retrieval.
	Save(ctx context.Context, data []byte, filename string, ownerID string) (string, error)

	// Get returns file bytes.
	Get(ctx context.Context, id string) ([]byte, error)

	// GetMetadata returns filename and owner id for a given file id.
	GetMetadata(ctx context.Context, id string) (string, string, error)
}
