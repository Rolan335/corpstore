package fileaccess

import "context"

// Repository defines access control for shared files (files_permissions table).
type Repository interface {
	Grant(ctx context.Context, fileID, userID string, ownerTG OwnerTGInfo) error
	HasAccess(ctx context.Context, fileID, userID string) (bool, error)
	ListSharedFiles(ctx context.Context, userID string) ([]SharedFile, error)
	ListGrantedUsers(ctx context.Context, fileID string) ([]GrantedUser, error)
	Revoke(ctx context.Context, fileID, userID string) error
	RevokeAllByFile(ctx context.Context, fileID string) error
}

type OwnerTGInfo struct {
	FirstName string
	LastName  string
	Username  string
}

// SharedFile is a lightweight view of a shared file.
type SharedFile struct {
	ID       string `json:"id"`
	Filename string `json:"filename"`
	OwnerID  string `json:"owner_id"`
	OwnerTGFirstName string `json:"owner_tg_first_name,omitempty"`
	OwnerTGLastName  string `json:"owner_tg_last_name,omitempty"`
	OwnerTGUsername  string `json:"owner_tg_username,omitempty"`
}

// GrantedUser is a user who has access to a file.
type GrantedUser struct {
	UserID   string `json:"user_id"`
	Username string `json:"username"`
}
