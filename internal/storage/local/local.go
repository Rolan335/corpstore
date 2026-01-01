package local

import (
	"context"
	"os"
	"path/filepath"

	"github.com/google/uuid"
)

// LocalStorage stores files on the local filesystem under a configured directory.
// Files are stored by a generated UUID; the original filename is kept in a companion .name file.
type LocalStorage struct {
	dir string
}

func NewLocalStorage(dir string) (*LocalStorage, error) {
	return &LocalStorage{dir: dir}, nil
}

// Save implements FileStore.Save(ctx, data []byte)
func (l *LocalStorage) Save(ctx context.Context, data []byte) (string, error) {
	id := uuid.NewString()
	path := filepath.Join(l.dir, id)
	f, err := os.Create(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	if _, err := f.Write(data); err != nil {
		return "", err
	}

	return id, nil
}

func (l *LocalStorage) Get(ctx context.Context, id string) ([]byte, error) {
	path := filepath.Join(l.dir, id)
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return b, nil
}

// Delete removes stored file by id
func (l *LocalStorage) Delete(ctx context.Context, id string) error {
	return os.Remove(filepath.Join(l.dir, id))
}
