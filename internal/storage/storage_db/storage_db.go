package metastorage

import (
	"context"

	stor "corpstore/internal/storage"
	"corpstore/internal/storage/meta"
)

// MetaStorage implements stor.Storage by composing a low-level FileStore and a MetaStore.
type MetaStorage struct {
	fs   stor.FileStore
	meta meta.Store
}

// NewDBStorage creates a new MetaStorage and returns it as stor.Storage
func NewMetaStorage(fs stor.FileStore, m meta.Store) stor.Storage {
	return &MetaStorage{fs: fs, meta: m}
}

func (s *MetaStorage) Save(ctx context.Context, data []byte, filename string, ownerID string) (string, error) {
	id, err := s.fs.Save(ctx, data)
	if err != nil {
		return "", err
	}
	if err := s.meta.SaveFileMetadata(ctx, id, filename, ownerID); err != nil {
		// attempt to delete stored file as compensation if supported
		if d, ok := s.fs.(interface {
			Delete(ctx context.Context, id string) error
		}); ok {
			_ = d.Delete(ctx, id)
		}
		return "", err
	}
	return id, nil
}

func (s *MetaStorage) Get(ctx context.Context, id string) ([]byte, error) {
	return s.fs.Get(ctx, id)
}

func (s *MetaStorage) GetMetadata(ctx context.Context, id string) (string, string, error) {
	return s.meta.GetFileMetadata(ctx, id)
}
