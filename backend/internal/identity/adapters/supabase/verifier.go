// Package supabase verifies Supabase access tokens.
package supabase

import (
	"context"
	"errors"
	"log/slog"

	"github.com/MicahParks/keyfunc/v3"
	"github.com/golang-jwt/jwt/v5"
)

// Verifier implements ports.TokenVerifier for Supabase JWTs. HS256 tokens
// are checked against the project's JWT secret; asymmetric tokens
// (ES256/RS256) against the project's JWKS endpoint.
type Verifier struct {
	secret []byte
	jwks   keyfunc.Keyfunc
}

// NewVerifier keeps the JWKS refreshed until ctx is done.
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
