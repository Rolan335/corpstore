package files

import (
	"context"
	"errors"
	"os"

	"github.com/jackc/pgx/v5"

	"corpstore/internal/filestore"
)

// Service coordinates raw file storage with metadata persistence.
type Service struct {
	store filestore.Store
	repo  Repository
}

var (
	ErrNotFound = errors.New("file not found")
	ErrForbidden = errors.New("forbidden")
)

func NewService(store filestore.Store, repo Repository) *Service {
	return &Service{store: store, repo: repo}
}

func (s *Service) Save(ctx context.Context, data []byte, filename, ownerID string) (string, error) {
	id, err := s.store.Save(ctx, data)
	if err != nil {
		return "", err
	}
	if err := s.repo.Create(ctx, id, filename, ownerID); err != nil {
		_ = s.store.Delete(ctx, id)
		return "", err
	}
	return id, nil
}

func (s *Service) Get(ctx context.Context, id string) ([]byte, error) {
	return s.store.Get(ctx, id)
}

func (s *Service) GetMeta(ctx context.Context, id string) (File, error) {
	return s.repo.GetByID(ctx, id)
}

// GetForOwner returns file meta and bytes if owner matches.
func (s *Service) GetForOwner(ctx context.Context, ownerID, id string) (File, []byte, error) {
	meta, err := s.repo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return File{}, nil, ErrNotFound
		}
		return File{}, nil, err
	}
	if meta.OwnerID != ownerID {
		return File{}, nil, ErrForbidden
	}
	b, err := s.store.Get(ctx, id)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return File{}, nil, ErrNotFound
		}
		return File{}, nil, err
	}
	return meta, b, nil
}

func (s *Service) ListByOwner(ctx context.Context, ownerID string) ([]File, error) {
	return s.repo.ListByOwner(ctx, ownerID)
}

// DeleteForOwner removes file metadata and bytes if owner matches.
func (s *Service) DeleteForOwner(ctx context.Context, ownerID, id string) (File, error) {
	meta, err := s.repo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return File{}, ErrNotFound
		}
		return File{}, err
	}
	if meta.OwnerID != ownerID {
		return File{}, ErrForbidden
	}

	if err := s.repo.DeleteByID(ctx, id); err != nil {
		return File{}, err
	}
	if err := s.store.Delete(ctx, id); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return meta, ErrNotFound
		}
		return meta, err
	}
	return meta, nil
}
