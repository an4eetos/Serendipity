// Package config loads the API's settings. It is the only place that reads
// the environment.
package config

import (
	"errors"
	"os"
	"strconv"
	"strings"
	"unicode/utf8"
)

const (
	defaultTermsVersion  = "draft-2026-10"
	defaultPublicWordCap = 2500
	// maxTermsVersionLength is the limit of profiles.terms_version.
	maxTermsVersionLength = 64
)

var (
	errTermsVersion  = errors.New("TERMS_VERSION must be 1 to 64 characters")
	errPublicWordCap = errors.New("PUBLIC_WORD_CAP must be a positive whole number")
)

type Config struct {
	HTTP      HTTP
	Database  Database
	Supabase  Supabase
	Anthropic Anthropic
	Terms     Terms
	Coverage  Coverage
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

// Terms is the current terms of service.
type Terms struct {
	// Version is the version readers must accept before they write.
	Version string
}

// Coverage limits how much of one work can be public.
type Coverage struct {
	// PublicWordCap is the default number of distinct words of one work
	// that may be public; a work's own override takes precedence.
	PublicWordCap int
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
	version, err := termsVersion(os.LookupEnv("TERMS_VERSION"))
	if err != nil {
		return c, err
	}
	c.Terms = Terms{Version: version}
	wordCap, err := publicWordCap(os.Getenv("PUBLIC_WORD_CAP"))
	if err != nil {
		return c, err
	}
	c.Coverage = Coverage{PublicWordCap: wordCap}
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

// termsVersion reads TERMS_VERSION: unset means the default. A set value is
// trimmed and must fit profiles.terms_version, which holds 1 to 64
// characters once accepted.
func termsVersion(raw string, set bool) (string, error) {
	if !set {
		return defaultTermsVersion, nil
	}
	v := strings.TrimSpace(raw)
	if v == "" || utf8.RuneCountInString(v) > maxTermsVersionLength {
		return "", errTermsVersion
	}
	return v, nil
}

// publicWordCap parses PUBLIC_WORD_CAP: unset means the default, anything
// else must be a whole number of at least 1.
func publicWordCap(raw string) (int, error) {
	if raw == "" {
		return defaultPublicWordCap, nil
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < 1 {
		return 0, errPublicWordCap
	}
	return n, nil
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
