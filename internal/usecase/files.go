package usecase

import (
	"context"
	"errors"
	"io"
	"mime/multipart"
	"strings"
	"time"

	"corpstore/internal/files"
	"corpstore/internal/users"
)

// Files handles file-related operations.
type Files struct {
	filesSvc *files.Service
	users    users.Repository
}

func NewFiles(filesSvc *files.Service, usersRepo users.Repository) *Files {
	return &Files{filesSvc: filesSvc, users: usersRepo}
}

type FileUpload struct {
	ID       string `json:"id"`
	Filename string `json:"filename"`
}

// SaveRaw saves a single file given raw bytes.
func (u *Files) SaveRaw(ctx context.Context, ownerID, filename string, data []byte) (string, error) {
	ownerID = strings.TrimSpace(ownerID)
	if ownerID == "" {
		return "", errors.New("invalid owner id")
	}
	filename = strings.TrimSpace(filename)
	if filename == "" {
		return "", errors.New("filename required")
	}
	exists, err := u.users.Exists(ctx, ownerID)
	if err != nil {
		return "", err
	}
	if !exists {
		return "", errors.New("owner not found")
	}
	return u.filesSvc.Save(ctx, data, filename, ownerID)
}

// UploadMultipart saves multiple files for ownerID.
func (u *Files) UploadMultipart(ctx context.Context, ownerID string, headers []*multipart.FileHeader) ([]FileUpload, error) {
	ownerID = strings.TrimSpace(ownerID)
	if ownerID == "" {
		return nil, errors.New("invalid owner id")
	}
	exists, err := u.users.Exists(ctx, ownerID)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, errors.New("owner not found")
	}
	if len(headers) == 0 {
		return nil, errors.New("no files provided")
	}

	resp := make([]FileUpload, 0, len(headers))
	for _, fh := range headers {
		f, err := fh.Open()
		if err != nil {
			return nil, err
		}
		buf, err := io.ReadAll(f)
		f.Close()
		if err != nil {
			return nil, err
		}

		saveCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		id, err := u.filesSvc.Save(saveCtx, buf, fh.Filename, ownerID)
		cancel()
		if err != nil {
			return nil, err
		}
		resp = append(resp, FileUpload{Filename: fh.Filename, ID: id})
	}
	return resp, nil
}

// GetForOwner returns file meta and bytes if owner matches.
func (u *Files) GetForOwner(ctx context.Context, ownerID, id string) (files.File, []byte, error) {
	ownerID = strings.TrimSpace(ownerID)
	if ownerID == "" {
		return files.File{}, nil, errors.New("invalid owner id")
	}
	return u.filesSvc.GetForOwner(ctx, ownerID, id)
}

func (u *Files) ListByOwner(ctx context.Context, ownerID string) ([]files.File, error) {
	ownerID = strings.TrimSpace(ownerID)
	if ownerID == "" {
		return nil, errors.New("invalid owner id")
	}
	return u.filesSvc.ListByOwner(ctx, ownerID)
}
