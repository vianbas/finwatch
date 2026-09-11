package auth

import (
	"context"
	"net/http"
	"strings"

	"github.com/vianbas/finwatch/apps/api/internal/platform/web"
)

type contextKey string

const claimsContextKey contextKey = "auth_claims"

// RequireAuth rejects requests without a valid, unexpired bearer token with
// 401, and otherwise stores the decoded Claims on the request context.
func RequireAuth(verifier *Verifier) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token, ok := bearerToken(r.Header.Get("Authorization"))
			if !ok {
				w.Header().Set("WWW-Authenticate", "Bearer")
				web.WriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "missing bearer token")
				return
			}
			claims, err := verifier.Verify(token)
			if err != nil {
				w.Header().Set("WWW-Authenticate", "Bearer")
				web.WriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "invalid or expired token")
				return
			}
			ctx := context.WithValue(r.Context(), claimsContextKey, claims)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// RequireRole rejects requests whose authenticated Claims do not carry the
// given role with 403. It must run behind RequireAuth.
func RequireRole(role Role) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims, ok := ClaimsFromContext(r.Context())
			if !ok || claims.Role != role {
				web.WriteError(w, http.StatusForbidden, "FORBIDDEN", "insufficient role")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// ClaimsFromContext returns the Claims stored by RequireAuth, if any.
func ClaimsFromContext(ctx context.Context) (Claims, bool) {
	claims, ok := ctx.Value(claimsContextKey).(Claims)
	return claims, ok
}

// bearerToken extracts the token from an Authorization header. The auth
// scheme ("Bearer") is matched case-insensitively per RFC 7235 section 2.1.
func bearerToken(header string) (string, bool) {
	scheme, token, found := strings.Cut(header, " ")
	if !found || !strings.EqualFold(scheme, "Bearer") {
		return "", false
	}
	token = strings.TrimSpace(token)
	if token == "" {
		return "", false
	}
	return token, true
}
