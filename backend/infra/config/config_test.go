package config

import (
	"os"
	"strings"
	"testing"
)

// setEnv sets the variables the test needs and unsets the others Load
// reads, restoring all of them when the test ends. A nil value unsets.
func setEnv(t *testing.T, vars map[string]*string) {
	t.Helper()
	base := map[string]*string{
		"DATABASE_URL":              ptr("postgresql://user@127.0.0.1:5432/db"),
		"SUPABASE_URL":              ptr("http://127.0.0.1:54321"),
		"SUPABASE_SERVICE_ROLE_KEY": ptr("test-only"),
		"TERMS_VERSION":             nil,
		"PUBLIC_WORD_CAP":           nil,
	}
	for k, v := range vars {
		base[k] = v
	}
	for k, v := range base {
		t.Setenv(k, "") // restores the original value after the test
		if v == nil {
			if err := os.Unsetenv(k); err != nil {
				t.Fatal(err)
			}
			continue
		}
		t.Setenv(k, *v)
	}
}

func ptr(s string) *string { return &s }

func TestLoadTermsVersion(t *testing.T) {
	tests := []struct {
		name    string
		value   *string
		want    string
		wantErr bool
	}{
		{"unset means the default", nil, "draft-2026-10", false},
		{"a version", ptr("2026-11-01"), "2026-11-01", false},
		{"trimmed", ptr("  v2  "), "v2", false},
		{"64 characters, counted as the database does", ptr(strings.Repeat("é", 64)), strings.Repeat("é", 64), false},
		{"set but empty", ptr(""), "", true},
		{"only spaces", ptr("   "), "", true},
		{"65 characters", ptr(strings.Repeat("a", 65)), "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setEnv(t, map[string]*string{"TERMS_VERSION": tt.value})
			c, err := Load()
			if tt.wantErr {
				if err == nil || err.Error() != "TERMS_VERSION must be 1 to 64 characters" {
					t.Errorf("Load() err = %v, want the TERMS_VERSION error", err)
				}
				return
			}
			if err != nil || c.Terms.Version != tt.want {
				t.Errorf("Load() = %q, %v; want %q", c.Terms.Version, err, tt.want)
			}
		})
	}
}

func TestLoadPublicWordCap(t *testing.T) {
	tests := []struct {
		name    string
		value   *string
		want    int
		wantErr bool
	}{
		{"unset means the default", nil, 2500, false},
		{"empty means the default", ptr(""), 2500, false},
		{"a cap", ptr("1200"), 1200, false},
		{"one word", ptr("1"), 1, false},
		{"zero", ptr("0"), 0, true},
		{"negative", ptr("-3"), 0, true},
		{"fraction", ptr("1.5"), 0, true},
		{"not a number", ptr("abc"), 0, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setEnv(t, map[string]*string{"PUBLIC_WORD_CAP": tt.value})
			c, err := Load()
			if tt.wantErr {
				if err == nil || err.Error() != "PUBLIC_WORD_CAP must be a positive whole number" {
					t.Errorf("Load() err = %v, want the PUBLIC_WORD_CAP error", err)
				}
				return
			}
			if err != nil || c.Coverage.PublicWordCap != tt.want {
				t.Errorf("Load() = %d, %v; want %d", c.Coverage.PublicWordCap, err, tt.want)
			}
		})
	}
}

func TestLoadStillRequiresTheDatabase(t *testing.T) {
	setEnv(t, map[string]*string{"DATABASE_URL": nil})
	if _, err := Load(); err == nil || !strings.Contains(err.Error(), "DATABASE_URL") {
		t.Errorf("Load() err = %v, want the missing DATABASE_URL reported", err)
	}
}
