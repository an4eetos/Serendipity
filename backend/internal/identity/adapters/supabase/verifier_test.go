package supabase

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func TestVerifyHS256(t *testing.T) {
	const secret = "test-secret"
	v := &Verifier{secret: []byte(secret)}
	sign := func(claims jwt.MapClaims, key string) string {
		s, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(key))
		if err != nil {
			t.Fatal(err)
		}
		return s
	}
	exp := time.Now().Add(time.Hour).Unix()

	tests := []struct {
		name    string
		token   string
		want    string
		wantErr bool
	}{
		{"valid", sign(jwt.MapClaims{"sub": "user-1", "aud": "authenticated", "exp": exp}, secret), "user-1", false},
		{"wrong secret", sign(jwt.MapClaims{"sub": "user-1", "aud": "authenticated", "exp": exp}, "other"), "", true},
		{"wrong audience", sign(jwt.MapClaims{"sub": "user-1", "aud": "anon", "exp": exp}, secret), "", true},
		{"no expiry", sign(jwt.MapClaims{"sub": "user-1", "aud": "authenticated"}, secret), "", true},
		{"expired", sign(jwt.MapClaims{"sub": "user-1", "aud": "authenticated", "exp": time.Now().Add(-time.Minute).Unix()}, secret), "", true},
		{"no subject", sign(jwt.MapClaims{"aud": "authenticated", "exp": exp}, secret), "", true},
		{"garbage", "not-a-token", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := v.Verify(tt.token)
			if (err != nil) != tt.wantErr || got != tt.want {
				t.Errorf("Verify() = %q, %v; want %q, error %v", got, err, tt.want, tt.wantErr)
			}
		})
	}

	if _, err := (&Verifier{}).Verify(sign(jwt.MapClaims{"sub": "u", "aud": "authenticated", "exp": exp}, secret)); err == nil {
		t.Error("an HS256 token must be rejected when no secret is configured")
	}
}
