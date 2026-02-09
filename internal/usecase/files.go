package usecase

import (
	"context"
	"errors"
	"io"
	"mime/multipart"
	"os"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"corpstore/internal/fileaccess"
	"corpstore/internal/files"
	"corpstore/internal/users"
)

var (
	ErrUserNotFound = errors.New("user not found")
)

// Files handles file-related operations.
type Files struct {
	filesSvc *files.Service
	users    users.Repository
	access   fileaccess.Repository
}

func NewFiles(filesSvc *files.Service, usersRepo users.Repository, accessRepo fileaccess.Repository) *Files {
	return &Files{filesSvc: filesSvc, users: usersRepo, access: accessRepo}
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

// GetForUser returns file meta and bytes if user is owner or has access.
func (u *Files) GetForUser(ctx context.Context, userID, id string) (files.File, []byte, error) {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return files.File{}, nil, errors.New("invalid user id")
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return files.File{}, nil, errors.New("invalid file id")
	}
	meta, err := u.filesSvc.GetMeta(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return files.File{}, nil, files.ErrNotFound
		}
		return files.File{}, nil, err
	}
	if meta.OwnerID != userID {
		ok, err := u.access.HasAccess(ctx, id, userID)
		if err != nil {
			return files.File{}, nil, err
		}
		if !ok {
			return files.File{}, nil, files.ErrForbidden
		}
	}
	b, err := u.filesSvc.Get(ctx, id)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return files.File{}, nil, files.ErrNotFound
		}
		return files.File{}, nil, err
	}
	return meta, b, nil
}

func (u *Files) ListByOwner(ctx context.Context, ownerID string) ([]files.File, error) {
	ownerID = strings.TrimSpace(ownerID)
	if ownerID == "" {
		return nil, errors.New("invalid owner id")
	}
	return u.filesSvc.ListByOwner(ctx, ownerID)
}

// ListShared returns files shared with the given user.
func (u *Files) ListShared(ctx context.Context, userID string) ([]fileaccess.SharedFile, error) {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return nil, errors.New("invalid user id")
	}
	return u.access.ListSharedFiles(ctx, userID)
}

// DeleteForOwner removes file if owner matches.
func (u *Files) DeleteForOwner(ctx context.Context, ownerID, id string) (files.File, error) {
	ownerID = strings.TrimSpace(ownerID)
	if ownerID == "" {
		return files.File{}, errors.New("invalid owner id")
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return files.File{}, errors.New("invalid file id")
	}
	meta, err := u.filesSvc.GetMeta(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return files.File{}, files.ErrNotFound
		}
		return files.File{}, err
	}
	if meta.OwnerID != ownerID {
		return files.File{}, files.ErrForbidden
	}
	if err := u.access.RevokeAllByFile(ctx, id); err != nil {
		return files.File{}, err
	}
	return u.filesSvc.DeleteForOwner(ctx, ownerID, id)
}

// ShareByUsername grants access to a file by username.
func (u *Files) ShareByUsername(ctx context.Context, ownerID, fileID, username string, ownerTG fileaccess.OwnerTGInfo) error {
	ownerID = strings.TrimSpace(ownerID)
	if ownerID == "" {
		return errors.New("invalid owner id")
	}
	fileID = strings.TrimSpace(fileID)
	if fileID == "" {
		return errors.New("invalid file id")
	}
	username = strings.TrimSpace(username)
	if username == "" {
		return errors.New("username required")
	}

	meta, err := u.filesSvc.GetMeta(ctx, fileID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return files.ErrNotFound
		}
		return err
	}
	if meta.OwnerID != ownerID {
		return files.ErrForbidden
	}

	uRec, err := u.users.GetByUsername(ctx, username)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) && !strings.HasPrefix(username, "@") {
			uRec, err = u.users.GetByUsername(ctx, "@"+username)
		}
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return ErrUserNotFound
			}
			return err
		}
	}
	return u.access.Grant(ctx, fileID, uRec.ID, ownerTG)
}

// ListGrantedUsers returns users that have access to the owner's file.
func (u *Files) ListGrantedUsers(ctx context.Context, ownerID, fileID string) ([]fileaccess.GrantedUser, error) {
	ownerID = strings.TrimSpace(ownerID)
	if ownerID == "" {
		return nil, errors.New("invalid owner id")
	}
	fileID = strings.TrimSpace(fileID)
	if fileID == "" {
		return nil, errors.New("invalid file id")
	}
	meta, err := u.filesSvc.GetMeta(ctx, fileID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, files.ErrNotFound
		}
		return nil, err
	}
	if meta.OwnerID != ownerID {
		return nil, files.ErrForbidden
	}
	return u.access.ListGrantedUsers(ctx, fileID)
}

// RevokeShare revokes access for userID.
func (u *Files) RevokeShare(ctx context.Context, ownerID, fileID, userID string) error {
	ownerID = strings.TrimSpace(ownerID)
	if ownerID == "" {
		return errors.New("invalid owner id")
	}
	fileID = strings.TrimSpace(fileID)
	if fileID == "" {
		return errors.New("invalid file id")
	}
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return errors.New("invalid user id")
	}
	meta, err := u.filesSvc.GetMeta(ctx, fileID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return files.ErrNotFound
		}
		return err
	}
	if meta.OwnerID != ownerID {
		return files.ErrForbidden
	}
	return u.access.Revoke(ctx, fileID, userID)
}

// RevokeShareByUsername revokes access by username.
func (u *Files) RevokeShareByUsername(ctx context.Context, ownerID, fileID, username string) error {
	ownerID = strings.TrimSpace(ownerID)
	if ownerID == "" {
		return errors.New("invalid owner id")
	}
	fileID = strings.TrimSpace(fileID)
	if fileID == "" {
		return errors.New("invalid file id")
	}
	username = strings.TrimSpace(username)
	if username == "" {
		return errors.New("username required")
	}
	uRec, err := u.users.GetByUsername(ctx, username)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) && !strings.HasPrefix(username, "@") {
			uRec, err = u.users.GetByUsername(ctx, "@"+username)
		}
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return ErrUserNotFound
			}
			return err
		}
	}
	return u.RevokeShare(ctx, ownerID, fileID, uRec.ID)
}
