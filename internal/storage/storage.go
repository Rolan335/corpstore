package storage

import (
	"context"
	"io"
)

// FileStore stores raw bytes and returns a generated UUID id
type FileStore interface {
	Save(ctx context.Context, r io.ReadSeeker) (string, error)
	Get(ctx context.Context, id string) (io.ReadCloser, error)
	Delete(ctx context.Context, id string) error
}

// Storage defines operations for saving and retrieving files (composed layer).
// Implementations combine FileStore and MetaStore for metadata.
type Storage interface {
	// Save persists file data and returns a generated id (UUID) for later retrieval.
	Save(ctx context.Context, r io.ReadSeeker, filename string, ownerID string) (string, error)

	// Get returns a ReadCloser for the stored file.
	Get(ctx context.Context, id string) (io.ReadCloser, error)

	// GetMetadata returns filename and owner id for a given file id.
	GetMetadata(ctx context.Context, id string) (string, string, error)
}
