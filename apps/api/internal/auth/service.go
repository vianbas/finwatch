package auth

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
)

// ErrUserNotFound is returned by UserStore when no user has the given email.
var ErrUserNotFound = errors.New("auth: user not found")

// ErrInvalidCredentials is returned by Service.Login for any failure that
// should not reveal whether the email or the password was wrong.
var ErrInvalidCredentials = errors.New("auth: invalid credentials")

// User is a persisted account used for login.
type User struct {
	ID           string
	Email        string
	PasswordHash string
	Role         Role
}

// UserStore is the persistence port for user accounts.
type UserStore interface {
	GetUserByEmail(ctx context.Context, email string) (User, error)
}

// Service implements the login use case: verify credentials, issue a token.
type Service struct {
	store  UserStore
	issuer *Issuer
}

// NewService constructs a Service.
func NewService(store UserStore, issuer *Issuer) *Service {
	return &Service{store: store, issuer: issuer}
}

// dummyPasswordHash is a bcrypt hash of a fixed, non-secret string, computed
// once on first use. Login compares against it for unknown emails so that an
// unknown-email attempt costs the same bcrypt comparison as a wrong-password
// attempt on a real account, closing a timing oracle that would otherwise let
// an attacker enumerate registered emails.
var dummyPasswordHash = sync.OnceValue(func() string {
	hash, err := HashPassword("auth-dummy-password-for-constant-time-comparison")
	if err != nil {
		// HashPassword only fails on bcrypt-internal errors (e.g. an
		// unreasonably long input); the fixed string above can never
		// trigger that, so this is unreachable in practice.
		panic(fmt.Sprintf("auth: hash dummy password: %v", err))
	}
	return hash
})

// Login verifies email/password and, on success, returns a signed access
// token and the authenticated user. Unknown email and wrong password both
// return ErrInvalidCredentials so the caller cannot distinguish them.
func (s *Service) Login(ctx context.Context, email, password string) (string, User, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	user, err := s.store.GetUserByEmail(ctx, email)
	if errors.Is(err, ErrUserNotFound) {
		// Run a bcrypt comparison against a dummy hash so this path costs
		// the same as a wrong-password match below, not a timing oracle.
		_ = VerifyPassword(dummyPasswordHash(), password)
		return "", User{}, ErrInvalidCredentials
	}
	if err != nil {
		return "", User{}, fmt.Errorf("auth: get user by email: %w", err)
	}

	if err := VerifyPassword(user.PasswordHash, password); err != nil {
		return "", User{}, ErrInvalidCredentials
	}

	token, err := s.issuer.Issue(user.ID, user.Email, user.Role)
	if err != nil {
		return "", User{}, fmt.Errorf("auth: issue token: %w", err)
	}
	return token, user, nil
}
