// Package auth implements JWT-based authentication and role-based access
// control: password hashing, token issuance/verification, the login service,
// and HTTP middleware that enforces them.
package auth

import "time"

// Role is a RBAC role. Only operator and admin exist in this issue.
type Role string

const (
	RoleOperator Role = "operator"
	RoleAdmin    Role = "admin"
)

// Claims is the decoded, verified content of an access token.
type Claims struct {
	UserID    string
	Email     string
	Role      Role
	IssuedAt  time.Time
	ExpiresAt time.Time
}
