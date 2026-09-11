package auth

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// ErrInvalidToken is returned for malformed tokens or signature mismatches.
var ErrInvalidToken = errors.New("auth: invalid token")

// ErrExpiredToken is returned when the token's exp claim is in the past.
var ErrExpiredToken = errors.New("auth: token expired")

// tokenClaims is the on-the-wire JWT claim set: sub/iat/exp via
// RegisteredClaims, plus email and role.
type tokenClaims struct {
	Email string `json:"email"`
	Role  string `json:"role"`
	jwt.RegisteredClaims
}

// Issuer signs short-lived HS256 access tokens.
type Issuer struct {
	secret []byte
	ttl    time.Duration
}

// NewIssuer constructs an Issuer. secret is the HS256 signing key; ttl is the
// access token lifetime (15 minutes per the architecture decision).
func NewIssuer(secret []byte, ttl time.Duration) *Issuer {
	return &Issuer{secret: secret, ttl: ttl}
}

// Issue signs a new access token for the given user.
func (i *Issuer) Issue(userID, email string, role Role) (string, error) {
	now := time.Now().UTC()
	claims := tokenClaims{
		Email: email,
		Role:  string(role),
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(i.ttl)),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(i.secret)
}

// Verifier validates HS256 access tokens and extracts their claims.
type Verifier struct {
	secret []byte
}

// NewVerifier constructs a Verifier using the same secret the Issuer signed with.
func NewVerifier(secret []byte) *Verifier {
	return &Verifier{secret: secret}
}

// Verify parses and validates tokenString, returning ErrExpiredToken or
// ErrInvalidToken on failure.
func (v *Verifier) Verify(tokenString string) (Claims, error) {
	var claims tokenClaims
	_, err := jwt.ParseWithClaims(tokenString, &claims, func(*jwt.Token) (interface{}, error) {
		return v.secret, nil
	}, jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Name}), jwt.WithExpirationRequired())
	if errors.Is(err, jwt.ErrTokenExpired) {
		return Claims{}, ErrExpiredToken
	}
	if err != nil {
		return Claims{}, ErrInvalidToken
	}
	if claims.IssuedAt == nil {
		return Claims{}, ErrInvalidToken
	}
	return Claims{
		UserID:    claims.Subject,
		Email:     claims.Email,
		Role:      Role(claims.Role),
		IssuedAt:  claims.IssuedAt.Time,
		ExpiresAt: claims.ExpiresAt.Time,
	}, nil
}
