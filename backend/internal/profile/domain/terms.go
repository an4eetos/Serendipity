package domain

import (
	kernel "github.com/an4eetos/serendipity/backend/internal/kernel/domain"
)

var (
	ErrTermsNotAccepted = kernel.Forbidden("accept the current terms of service first")
	ErrTermsOutdated    = kernel.Conflict("those terms are not the current version; review the current terms and accept them")
)

// Terms is the current version of the terms of service, from configuration.
// A reader must accept it before they change anything.
type Terms struct {
	Version string
}

// AcceptedBy reports whether p accepted this version of the terms.
func (t Terms) AcceptedBy(p Profile) bool {
	return p.TermsAcceptedAt != nil && p.TermsVersion == t.Version
}

// CheckAccepted returns ErrTermsNotAccepted unless p accepted this version.
func (t Terms) CheckAccepted(p Profile) error {
	if !t.AcceptedBy(p) {
		return ErrTermsNotAccepted
	}
	return nil
}

// CheckAcceptable returns ErrTermsOutdated unless version is this version: a
// reader can only accept the terms they are shown now.
func (t Terms) CheckAcceptable(version string) error {
	if version != t.Version {
		return ErrTermsOutdated
	}
	return nil
}

// Me is the signed-in reader's own profile and where they stand on the terms.
type Me struct {
	Profile       Profile
	CurrentTerms  Terms
	TermsAccepted bool
}

// NewMe tells the reader p whether they accepted the current terms.
func NewMe(p Profile, current Terms) Me {
	return Me{Profile: p, CurrentTerms: current, TermsAccepted: current.AcceptedBy(p)}
}
