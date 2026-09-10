package auth_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/vianbas/finwatch/apps/api/internal/auth"
)

type fakeUserStore struct {
	usersByEmail map[string]auth.User
}

func (f *fakeUserStore) GetUserByEmail(_ context.Context, email string) (auth.User, error) {
	u, ok := f.usersByEmail[email]
	if !ok {
		return auth.User{}, auth.ErrUserNotFound
	}
	return u, nil
}

func newTestService(t *testing.T, users ...auth.User) *auth.Service {
	t.Helper()
	byEmail := make(map[string]auth.User, len(users))
	for _, u := range users {
		byEmail[u.Email] = u
	}
	issuer := auth.NewIssuer([]byte("test-signing-secret-must-be-at-least-32-bytes"), 15*time.Minute)
	return auth.NewService(&fakeUserStore{usersByEmail: byEmail}, issuer)
}

func userWithPassword(t *testing.T, email, password string, role auth.Role) auth.User {
	t.Helper()
	hash, err := auth.HashPassword(password)
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	return auth.User{ID: "user-1", Email: email, PasswordHash: hash, Role: role}
}

func TestService_Login_ValidCredentials(t *testing.T) {
	user := userWithPassword(t, "operator@example.com", "correct-password", auth.RoleOperator)
	svc := newTestService(t, user)

	token, got, err := svc.Login(context.Background(), "operator@example.com", "correct-password")
	if err != nil {
		t.Fatalf("Login: %v", err)
	}
	if token == "" {
		t.Errorf("want non-empty token")
	}
	if got.Email != user.Email || got.Role != user.Role {
		t.Errorf("got user %+v, want email/role to match %+v", got, user)
	}
}

func TestService_Login_WrongPassword(t *testing.T) {
	user := userWithPassword(t, "operator@example.com", "correct-password", auth.RoleOperator)
	svc := newTestService(t, user)

	_, _, err := svc.Login(context.Background(), "operator@example.com", "wrong-password")
	if !errors.Is(err, auth.ErrInvalidCredentials) {
		t.Fatalf("got %v, want ErrInvalidCredentials", err)
	}
}

func TestService_Login_UnknownEmail(t *testing.T) {
	svc := newTestService(t)

	_, _, err := svc.Login(context.Background(), "nobody@example.com", "whatever")
	if !errors.Is(err, auth.ErrInvalidCredentials) {
		t.Fatalf("got %v, want ErrInvalidCredentials", err)
	}
}
