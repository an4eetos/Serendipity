// Package config loads the API's settings. It is the only place that reads
// the environment.
package config

import (
	"errors"
	"os"
	"strings"
)

type Config struct {
	HTTP      HTTP
	Database  Database
	Supabase  Supabase
	Anthropic Anthropic
}

type HTTP struct {
	Addr           string
	AllowedOrigins []string
}

type Database struct {
	URL string
}

type Supabase struct {
	URL            string
	ServiceRoleKey string
	// JWTSecret verifies HS256 access tokens (legacy/local projects).
	// Projects using asymmetric signing keys are verified via JWKS instead.
	JWTSecret string
}

type Anthropic struct {
	// APIKey may be empty at startup; characters can't answer without it.
	APIKey string
	// Model is the model the characters run on.
	Model string
}

// Load reads the configuration from the environment.
func Load() (Config, error) {
	c := Config{
		HTTP: HTTP{
			Addr:           env("API_ADDR", ":8787"),
			AllowedOrigins: strings.Split(env("ALLOWED_ORIGINS", "http://localhost:3000,http://127.0.0.1:3000"), ","),
		},
		Database: Database{URL: os.Getenv("DATABASE_URL")},
		Supabase: Supabase{
			URL:            strings.TrimRight(os.Getenv("SUPABASE_URL"), "/"),
			ServiceRoleKey: os.Getenv("SUPABASE_SERVICE_ROLE_KEY"),
			JWTSecret:      os.Getenv("SUPABASE_JWT_SECRET"),
		},
		Anthropic: Anthropic{
			APIKey: os.Getenv("ANTHROPIC_API_KEY"),
			Model:  env("ANTHROPIC_MODEL", "claude-opus-5-5"),
		},
	}
	var missing []string
	for _, v := range []struct{ name, value string }{
		{"DATABASE_URL", c.Database.URL},
		{"SUPABASE_URL", c.Supabase.URL},
		{"SUPABASE_SERVICE_ROLE_KEY", c.Supabase.ServiceRoleKey},
	} {
		if v.value == "" {
			missing = append(missing, v.name)
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
