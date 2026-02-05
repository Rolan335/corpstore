package files

import "time"

// File represents a row in files table.
type File struct {
	ID        string    `json:"id"`
	Filename  string    `json:"filename"`
	OwnerID   string    `json:"owner_id"`
	CreatedAt time.Time `json:"created_at,omitempty"`
}
