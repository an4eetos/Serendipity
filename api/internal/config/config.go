// Package config loads the API's settings from the environment.
package config

import (
	"errors"
	"os"
	"strings"
)

type Config struct {
	Addr        string
	DatabaseURL string

	SupabaseURL            string
	SupabaseServiceRoleKey string
	// SupabaseJWTSecret verifies HS256 access tokens (legacy/local projects).
	// Projects using asymmetric signing keys are verified via JWKS instead.
	SupabaseJWTSecret string

	// AnthropicModel is the model the characters run on. The API key is read
	// by the Anthropic SDK from ANTHROPIC_API_KEY.
	AnthropicModel string

	AllowedOrigins []string
}

func Load() (Config, error) {
	c := Config{
		Addr:                   env("API_ADDR", ":8787"),
		DatabaseURL:            os.Getenv("DATABASE_URL"),
		SupabaseURL:            strings.TrimRight(os.Getenv("SUPABASE_URL"), "/"),
		SupabaseServiceRoleKey: os.Getenv("SUPABASE_SERVICE_ROLE_KEY"),
		SupabaseJWTSecret:      os.Getenv("SUPABASE_JWT_SECRET"),
		AnthropicModel:         env("ANTHROPIC_MODEL", "claude-opus-5-5"),
		AllowedOrigins:         strings.Split(env("ALLOWED_ORIGINS", "http://localhost:3000,http://127.0.0.1:3000"), ","),
	}
	var missing []string
	for name, v := range map[string]string{
		"DATABASE_URL":              c.DatabaseURL,
		"SUPABASE_URL":              c.SupabaseURL,
		"SUPABASE_SERVICE_ROLE_KEY": c.SupabaseServiceRoleKey,
	} {
		if v == "" {
			missing = append(missing, name)
		}
	}
	if len(missing) > 0 {
		return c, errors.New("missing required env: " + strings.Join(missing, ", "))
	}
	return c, nil
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
