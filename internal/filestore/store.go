package filestore

import "context"

// Store persists raw file bytes and returns generated IDs.
type Store interface {
	Save(ctx context.Context, data []byte) (string, error)
	Get(ctx context.Context, id string) ([]byte, error)
	Delete(ctx context.Context, id string) error
}
