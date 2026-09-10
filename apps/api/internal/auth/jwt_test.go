package auth

import (
	"errors"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const testSecret = "test-signing-secret-must-be-at-least-32-bytes"

func TestIssueAndVerify_RoundTrip(t *testing.T) {
	issuer := NewIssuer([]byte(testSecret), 15*time.Minute)
	verifier := NewVerifier([]byte(testSecret))

	token, err := issuer.Issue("user-1", "operator@example.com", RoleOperator)
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}

	claims, err := verifier.Verify(token)
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if claims.UserID != "user-1" {
		t.Errorf("UserID = %q, want user-1", claims.UserID)
	}
	if claims.Email != "operator@example.com" {
		t.Errorf("Email = %q, want operator@example.com", claims.Email)
	}
	if claims.Role != RoleOperator {
		t.Errorf("Role = %q, want operator", claims.Role)
	}
	if !claims.ExpiresAt.After(claims.IssuedAt) {
		t.Errorf("ExpiresAt %v must be after IssuedAt %v", claims.ExpiresAt, claims.IssuedAt)
	}
}

func TestVerify_ExpiredToken(t *testing.T) {
	issuer := NewIssuer([]byte(testSecret), -1*time.Minute) // already expired
	verifier := NewVerifier([]byte(testSecret))

	token, err := issuer.Issue("user-1", "operator@example.com", RoleOperator)
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}
	if _, err := verifier.Verify(token); !errors.Is(err, ErrExpiredToken) {
		t.Fatalf("got %v, want ErrExpiredToken", err)
	}
}

func TestVerify_WrongSignature(t *testing.T) {
	issuer := NewIssuer([]byte(testSecret), 15*time.Minute)
	verifier := NewVerifier([]byte("a-completely-different-32-byte-secret!"))

	token, err := issuer.Issue("user-1", "operator@example.com", RoleOperator)
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}
	if _, err := verifier.Verify(token); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("got %v, want ErrInvalidToken", err)
	}
}

func TestVerify_MalformedToken(t *testing.T) {
	verifier := NewVerifier([]byte(testSecret))
	if _, err := verifier.Verify("not-a-jwt"); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("got %v, want ErrInvalidToken", err)
	}
}

func TestVerify_MissingExpiration(t *testing.T) {
	verifier := NewVerifier([]byte(testSecret))

	claims := tokenClaims{
		Email: "operator@example.com",
		Role:  string(RoleOperator),
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:  "user-1",
			IssuedAt: jwt.NewNumericDate(time.Now().UTC()),
			// ExpiresAt intentionally omitted.
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte(testSecret))
	if err != nil {
		t.Fatalf("SignedString: %v", err)
	}

	if _, err := verifier.Verify(signed); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("got %v, want ErrInvalidToken", err)
	}
}

func TestVerify_MissingIssuedAt(t *testing.T) {
	verifier := NewVerifier([]byte(testSecret))

	claims := tokenClaims{
		Email: "operator@example.com",
		Role:  string(RoleOperator),
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   "user-1",
			ExpiresAt: jwt.NewNumericDate(time.Now().UTC().Add(15 * time.Minute)),
			// IssuedAt intentionally omitted.
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte(testSecret))
	if err != nil {
		t.Fatalf("SignedString: %v", err)
	}

	if _, err := verifier.Verify(signed); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("got %v, want ErrInvalidToken", err)
	}
}
