package local

import (
	"context"
	"io"
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
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	return &LocalStorage{dir: dir}, nil
}

func newUUID() (string, error) {
	return uuid.NewString(), nil
}

// Save implements FileStore.Save(ctx, r)
func (l *LocalStorage) Save(ctx context.Context, r io.ReadSeeker) (string, error) {
	id, err := newUUID()
	if err != nil {
		return "", err
	}
	path := filepath.Join(l.dir, id)
	f, err := os.Create(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	// ensure reader is at start
	if _, err := r.Seek(0, io.SeekStart); err == nil {
		// copy
		if _, err := io.Copy(f, r); err != nil {
			return "", err
		}
	} else {
		// If Seek not supported, still try copying from current position
		if _, err := io.Copy(f, r); err != nil {
			return "", err
		}
	}

	return id, nil
}

func (l *LocalStorage) Get(ctx context.Context, id string) (io.ReadCloser, error) {
	path := filepath.Join(l.dir, id)
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	return f, nil
}

// Delete removes stored file by id
func (l *LocalStorage) Delete(ctx context.Context, id string) error {
	return os.Remove(filepath.Join(l.dir, id))
}
