package auth

import "time"

// Role represents an API key permission level.
type Role string

const (
	RoleAdmin     Role = "admin"
	RoleReadWrite Role = "read_write"
	RoleReadOnly  Role = "read_only"
)

// APIKey represents a stored API key record.
type APIKey struct {
	ID        string
	ProjectID *string
	Name      string
	TokenHash string
	Role      Role
	CreatedAt time.Time
	RevokedAt *time.Time
}

// AuthContext holds the authenticated identity for a request.
type AuthContext struct {
	KeyID     string
	ProjectID *string
	Role      Role
}
