package local

import (
	"context"
	"os"
	"path/filepath"

	"github.com/google/uuid"
)

// LocalStore stores files on the local filesystem under a configured directory.
type LocalStore struct {
	dir string
}

func New(dir string) (*LocalStore, error) {
	return &LocalStore{dir: dir}, nil
}

func (l *LocalStore) Save(ctx context.Context, data []byte) (string, error) {
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

func (l *LocalStore) Get(ctx context.Context, id string) ([]byte, error) {
	path := filepath.Join(l.dir, id)
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return b, nil
}

func (l *LocalStore) Delete(ctx context.Context, id string) error {
	return os.Remove(filepath.Join(l.dir, id))
}
