package users

// User represents a row in users table.
type User struct {
	ID           string
	Username     string
	PasswordHash string
}
