package usecase

import (
	"errors"
	"testing"
	"time"

	kernel "github.com/an4eetos/serendipity/backend/internal/kernel/domain"
	"github.com/an4eetos/serendipity/backend/internal/kernel/ports/portstest"
	"github.com/an4eetos/serendipity/backend/internal/profile/domain"
)

var current = domain.Terms{Version: "v2"}

// readers holds one reader per stance on the terms.
func readers() *fakeProfiles {
	accepted := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	return &fakeProfiles{byID: map[string]domain.Profile{
		"new":      {ID: "new"},
		"outdated": {ID: "outdated", TermsVersion: "v1", TermsAcceptedAt: &accepted},
		"current":  {ID: "current", TermsVersion: "v2", TermsAcceptedAt: &accepted},
	}}
}

func TestCheckTerms(t *testing.T) {
	tests := []struct {
		reader string
		want   error
	}{
		{"new", domain.ErrTermsNotAccepted},
		{"outdated", domain.ErrTermsNotAccepted},
		{"current", nil},
		{"missing", kernel.ErrNotFound},
	}
	for _, tt := range tests {
		t.Run(tt.reader, func(t *testing.T) {
			if err := NewCheckTerms(readers(), current).Execute(t.Context(), tt.reader); !errors.Is(err, tt.want) {
				t.Errorf("err = %v, want %v", err, tt.want)
			}
		})
	}
}

func TestAcceptTerms(t *testing.T) {
	t.Run("accepts the current version in a transaction", func(t *testing.T) {
		profiles, txm := readers(), &portstest.TxManager{}
		me, err := NewAcceptTerms(txm, profiles, current).Execute(t.Context(), "outdated", "v2")
		if err != nil {
			t.Fatal(err)
		}
		if !me.TermsAccepted || me.Profile.TermsVersion != "v2" || me.CurrentTerms != current {
			t.Errorf("Execute() = %#v", me)
		}
		if txm.Calls != 1 || profiles.outsideTx {
			t.Errorf("transactions = %d, written outside one: %v", txm.Calls, profiles.outsideTx)
		}
		// The gate lets the reader through now.
		if err := NewCheckTerms(profiles, current).Execute(t.Context(), "outdated"); err != nil {
			t.Errorf("CheckTerms after accepting: %v", err)
		}
	})

	t.Run("refuses a version that isn't the current one", func(t *testing.T) {
		for _, version := range []string{"v1", "", "V2", "v3"} {
			profiles, txm := readers(), &portstest.TxManager{}
			_, err := NewAcceptTerms(txm, profiles, current).Execute(t.Context(), "new", version)
			if !errors.Is(err, domain.ErrTermsOutdated) || txm.Calls != 0 || len(profiles.accepted) > 0 {
				t.Errorf("version %q: err = %v, transactions %d, accepted %v", version, err, txm.Calls, profiles.accepted)
			}
		}
	})

	t.Run("missing profile", func(t *testing.T) {
		_, err := NewAcceptTerms(&portstest.TxManager{}, readers(), current).Execute(t.Context(), "missing", "v2")
		if !errors.Is(err, kernel.ErrNotFound) {
			t.Errorf("err = %v", err)
		}
	})
}

func TestGetMe(t *testing.T) {
	tests := []struct {
		reader       string
		wantAccepted bool
	}{
		{"new", false},
		{"outdated", false},
		{"current", true},
	}
	for _, tt := range tests {
		t.Run(tt.reader, func(t *testing.T) {
			me, err := NewGetMe(readers(), current).Execute(t.Context(), tt.reader)
			if err != nil || me.Profile.ID != tt.reader || me.CurrentTerms != current || me.TermsAccepted != tt.wantAccepted {
				t.Errorf("Execute() = %#v, %v; want accepted %v", me, err, tt.wantAccepted)
			}
		})
	}
	if _, err := NewGetMe(readers(), current).Execute(t.Context(), "missing"); !errors.Is(err, kernel.ErrNotFound) {
		t.Errorf("missing profile: err = %v", err)
	}
}

func TestGetTerms(t *testing.T) {
	if got := NewGetTerms(current).Execute(t.Context()); got != current {
		t.Errorf("Execute() = %#v, want %#v", got, current)
	}
}
