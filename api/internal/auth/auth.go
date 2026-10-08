// Package auth verifies Supabase access tokens.
package auth

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/MicahParks/keyfunc/v3"
	"github.com/golang-jwt/jwt/v5"
)

type ctxKey struct{}

// Verifier checks Supabase JWTs. HS256 tokens are checked against the
// project's JWT secret; asymmetric tokens (ES256/RS256) against the project's
// JWKS endpoint.
type Verifier struct {
	secret []byte
	jwks   keyfunc.Keyfunc
}

func NewVerifier(ctx context.Context, supabaseURL, jwtSecret string) *Verifier {
	v := &Verifier{secret: []byte(jwtSecret)}
	jwks, err := keyfunc.NewDefaultCtx(ctx, []string{supabaseURL + "/auth/v1/.well-known/jwks.json"})
	if err != nil {
		slog.Warn("JWKS unavailable; only HS256 tokens will be accepted", "err", err)
	} else {
		v.jwks = jwks
	}
	return v
}

func (v *Verifier) keyFunc(t *jwt.Token) (any, error) {
	if _, ok := t.Method.(*jwt.SigningMethodHMAC); ok {
		if len(v.secret) == 0 {
			return nil, errors.New("HS256 token but SUPABASE_JWT_SECRET is not set")
		}
		return v.secret, nil
	}
	if v.jwks == nil {
		return nil, errors.New("asymmetric token but JWKS is unavailable")
	}
	return v.jwks.Keyfunc(t)
}

// Verify returns the user id (the "sub" claim) of a valid access token.
func (v *Verifier) Verify(token string) (string, error) {
	claims := jwt.MapClaims{}
	_, err := jwt.ParseWithClaims(token, claims, v.keyFunc,
		jwt.WithValidMethods([]string{"HS256", "ES256", "RS256"}),
		jwt.WithAudience("authenticated"),
		jwt.WithExpirationRequired(),
	)
	if err != nil {
		return "", err
	}
	sub, err := claims.GetSubject()
	if err != nil || sub == "" {
		return "", errors.New("token has no subject")
	}
	return sub, nil
}

func bearer(r *http.Request) string {
	h := r.Header.Get("Authorization")
	if t, ok := strings.CutPrefix(h, "Bearer "); ok {
		return strings.TrimSpace(t)
	}
	return ""
}

// Require rejects requests without a valid access token.
func (v *Verifier) Require(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		uid, err := v.Verify(bearer(r))
		if err != nil {
			http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), ctxKey{}, uid)))
	})
}

// Optional attaches the user id when a valid token is present.
func (v *Verifier) Optional(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if t := bearer(r); t != "" {
			if uid, err := v.Verify(t); err == nil {
				r = r.WithContext(context.WithValue(r.Context(), ctxKey{}, uid))
			}
		}
		next.ServeHTTP(w, r)
	})
}

// UserID returns the authenticated user's id, or "" if anonymous.
func UserID(ctx context.Context) string {
	uid, _ := ctx.Value(ctxKey{}).(string)
	return uid
}
