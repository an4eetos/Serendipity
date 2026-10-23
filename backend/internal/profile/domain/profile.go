// Package domain holds the rules of readers' profiles: what a reader is
// called, and what anyone may see of what they read.
package domain

import (
	"regexp"
	"strings"
	"time"

	kernel "github.com/an4eetos/serendipity/backend/internal/kernel/domain"
)

var usernameRe = regexp.MustCompile(`^[a-z0-9_]{3,30}$`)

var (
	ErrInvalidUsername = kernel.Invalid("username must be 3–30 characters: a–z, 0–9 or _")
	ErrUsernameTaken   = kernel.Conflict("that username is taken")
	ErrInvalidField    = kernel.Invalid("invalid profile field")
)

type Profile struct {
	ID          string
	Username    string
	DisplayName string
	Bio         string
	IsPublic    bool
	// TermsVersion is the version of the terms of service the reader last
	// accepted; "" until they accept one.
	TermsVersion string
	// TermsAcceptedAt is when they accepted it; nil until then.
	TermsAcceptedAt *time.Time
	CreatedAt       time.Time
}

// VisibleTo reports whether viewerID ("" if anonymous) may see the profile:
// private profiles are visible only to their owner.
func (p Profile) VisibleTo(viewerID string) bool {
	return p.IsPublic || p.ID == viewerID
}

// NormalizeUsername is the form usernames are stored and looked up in.
func NormalizeUsername(username string) string {
	return strings.ToLower(strings.TrimSpace(username))
}

// Patch changes a profile; nil fields stay as they are.
type Patch struct {
	Username    *string
	DisplayName *string
	Bio         *string
	IsPublic    *bool
}

// Normalize checks the patch and normalizes its username.
func (p Patch) Normalize() (Patch, error) {
	if p.Username != nil {
		u := NormalizeUsername(*p.Username)
		if !usernameRe.MatchString(u) {
			return Patch{}, ErrInvalidUsername
		}
		p.Username = &u
	}
	return p, nil
}
