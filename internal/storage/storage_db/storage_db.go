package storage_db

import (
	"context"

	stor "corpstore/internal/storage"
	"corpstore/internal/storage/meta"
)

// DBStorage implements stor.Storage by composing a low-level FileStore and a MetaStore.
type DBStorage struct {
	fs   stor.FileStore
	meta meta.MetaStore
}

// NewDBStorage creates a new DBStorage and returns it as stor.Storage
func NewDBStorage(fs stor.FileStore, m meta.MetaStore) stor.Storage {
	return &DBStorage{fs: fs, meta: m}
}

func (s *DBStorage) Save(ctx context.Context, data []byte, filename string, ownerID string) (string, error) {
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

func (s *DBStorage) Get(ctx context.Context, id string) ([]byte, error) {
	return s.fs.Get(ctx, id)
}

func (s *DBStorage) GetMetadata(ctx context.Context, id string) (string, string, error) {
	return s.meta.GetFileMetadata(ctx, id)
}
