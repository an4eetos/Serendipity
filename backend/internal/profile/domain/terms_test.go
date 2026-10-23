package domain

import (
	"errors"
	"testing"
	"time"

	kernel "github.com/an4eetos/serendipity/backend/internal/kernel/domain"
)

func TestTermsAcceptedBy(t *testing.T) {
	accepted := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	current := Terms{Version: "v2"}
	tests := []struct {
		name    string
		profile Profile
		want    bool
	}{
		{"never accepted", Profile{}, false},
		{"accepted an older version", Profile{TermsVersion: "v1", TermsAcceptedAt: &accepted}, false},
		{"accepted the current version", Profile{TermsVersion: "v2", TermsAcceptedAt: &accepted}, true},
		{"version without an acceptance time", Profile{TermsVersion: "v2"}, false},
		{"versions match exactly", Profile{TermsVersion: "V2", TermsAcceptedAt: &accepted}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := current.AcceptedBy(tt.profile); got != tt.want {
				t.Errorf("AcceptedBy() = %v, want %v", got, tt.want)
			}
			err := current.CheckAccepted(tt.profile)
			if tt.want && err != nil || !tt.want && !errors.Is(err, ErrTermsNotAccepted) {
				t.Errorf("CheckAccepted() = %v", err)
			}
			if me := NewMe(tt.profile, current); me.TermsAccepted != tt.want || me.CurrentTerms != current {
				t.Errorf("NewMe() = %#v", me)
			}
		})
	}
	// Raising the version asks everyone again.
	if (Terms{Version: "v3"}).AcceptedBy(Profile{TermsVersion: "v2", TermsAcceptedAt: &accepted}) {
		t.Error("a reader who accepted v2 must accept v3 again")
	}
}

func TestTermsCheckAcceptable(t *testing.T) {
	current := Terms{Version: "draft-2026-10"}
	if err := current.CheckAcceptable("draft-2026-10"); err != nil {
		t.Errorf("current version: %v", err)
	}
	for _, v := range []string{"", "draft-2026-09", "draft-2026-10 ", "DRAFT-2026-10"} {
		if err := current.CheckAcceptable(v); !errors.Is(err, ErrTermsOutdated) {
			t.Errorf("CheckAcceptable(%q) = %v, want ErrTermsOutdated", v, err)
		}
	}
}

func TestTermsErrorKinds(t *testing.T) {
	var de kernel.DomainError
	if !errors.As(ErrTermsNotAccepted, &de) || de.Kind() != kernel.KindForbidden {
		t.Errorf("ErrTermsNotAccepted must be Forbidden (403), got %v", de)
	}
	if !errors.As(ErrTermsOutdated, &de) || de.Kind() != kernel.KindConflict {
		t.Errorf("ErrTermsOutdated must be a Conflict (409), got %v", de)
	}
}
