package fileaccess

import "context"

// Repository defines access control for shared files (files_permissions table).
type Repository interface {
	Grant(ctx context.Context, fileID, userID string) error
	HasAccess(ctx context.Context, fileID, userID string) (bool, error)
	ListSharedFiles(ctx context.Context, userID string) ([]SharedFile, error)
}

// SharedFile is a lightweight view of a shared file.
type SharedFile struct {
	ID       string `json:"id"`
	Filename string `json:"filename"`
	OwnerID  string `json:"owner_id"`
}
