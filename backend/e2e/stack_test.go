//go:build e2e

package e2e

import (
	"bufio"
	"context"
	"errors"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// anonKeyTimeout bounds `npx supabase status`, which may first download the CLI.
const anonKeyTimeout = 3 * time.Minute

// stackConfig is where the local stack lives and how to reach it.
type stackConfig struct {
	backendDir     string
	supabaseURL    string
	databaseURL    string
	serviceRoleKey string
	anonKey        string
	// serverEnv is the environment for the API binary, before the test's
	// own overrides are appended.
	serverEnv []string
}

// loadStack reads backend/.env (process environment variables take
// precedence), refuses anything but a local stack, and finds the anon key.
func loadStack(t *testing.T) stackConfig {
	t.Helper()
	backendDir, err := filepath.Abs("..")
	if err != nil {
		t.Fatal(err)
	}
	dotenv, err := parseDotEnv(filepath.Join(backendDir, ".env"))
	if err != nil {
		t.Fatalf("read backend/.env: %v", err)
	}
	lookup := func(key string) string {
		if v := os.Getenv(key); v != "" {
			return v
		}
		return dotenv[key]
	}
	c := stackConfig{
		backendDir:     backendDir,
		supabaseURL:    strings.TrimRight(lookup("SUPABASE_URL"), "/"),
		databaseURL:    lookup("DATABASE_URL"),
		serviceRoleKey: lookup("SUPABASE_SERVICE_ROLE_KEY"),
	}
	for name, v := range map[string]string{
		"SUPABASE_URL": c.supabaseURL, "DATABASE_URL": c.databaseURL, "SUPABASE_SERVICE_ROLE_KEY": c.serviceRoleKey,
	} {
		if v == "" {
			t.Fatalf("%s is not set in backend/.env or the environment", name)
		}
	}
	requireLocal(t, "SUPABASE_URL", c.supabaseURL)
	requireLocal(t, "DATABASE_URL", c.databaseURL)

	c.serverEnv = os.Environ()
	for k, v := range dotenv {
		if os.Getenv(k) == "" {
			c.serverEnv = append(c.serverEnv, k+"="+v)
		}
	}
	// The binary must talk to exactly the stack that was checked above.
	c.serverEnv = append(c.serverEnv, "SUPABASE_URL="+c.supabaseURL, "DATABASE_URL="+c.databaseURL)

	c.anonKey = lookup("SUPABASE_ANON_KEY")
	if c.anonKey == "" {
		c.anonKey = anonKeyFromCLI(t, filepath.Dir(backendDir))
	}
	return c
}

// requireLocal stops the test unless raw points at this machine.
func requireLocal(t *testing.T, name, raw string) {
	t.Helper()
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatalf("refusing to run: %s is not a URL", name)
	}
	switch u.Hostname() {
	case "127.0.0.1", "localhost", "::1":
	default:
		t.Fatalf("refusing to run: %s must point at 127.0.0.1 or localhost, not %q", name, u.Hostname())
	}
}

// anonKeyFromCLI reads ANON_KEY from `npx supabase status -o env`, run in
// the repository root. The output holds secrets, so it is never logged.
func anonKeyFromCLI(t *testing.T, repoRoot string) string {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), anonKeyTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "npx", "--yes", "supabase", "status", "-o", "env")
	cmd.Dir = repoRoot
	out, err := cmd.Output()
	if err != nil {
		// Only stderr is shown: stdout carries the keys.
		var stderr []byte
		if ee, ok := errors.AsType[*exec.ExitError](err); ok {
			stderr = ee.Stderr
		}
		t.Fatalf("SUPABASE_ANON_KEY is unset and `npx supabase status -o env` in %s failed: %v\n%s",
			repoRoot, err, snippet(stderr))
	}
	vars := parseEnvLines(string(out))
	if vars["ANON_KEY"] == "" {
		t.Fatal("SUPABASE_ANON_KEY is unset and `npx supabase status -o env` printed no ANON_KEY")
	}
	return vars["ANON_KEY"]
}

// parseDotEnv reads KEY=VALUE lines. A missing file is an empty map.
func parseDotEnv(path string) (map[string]string, error) {
	raw, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return map[string]string{}, nil
	}
	if err != nil {
		return nil, err
	}
	return parseEnvLines(string(raw)), nil
}

// parseEnvLines parses dotenv-style text: blank lines and # comments are
// skipped, an "export " prefix is allowed, and one pair of surrounding
// quotes is removed from the value.
func parseEnvLines(text string) map[string]string {
	vars := map[string]string{}
	sc := bufio.NewScanner(strings.NewReader(text))
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		line = strings.TrimPrefix(line, "export ")
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		value = strings.TrimSpace(value)
		if len(value) >= 2 && (value[0] == '"' || value[0] == '\'') && value[len(value)-1] == value[0] {
			value = value[1 : len(value)-1]
		}
		vars[strings.TrimSpace(key)] = value
	}
	return vars
}
