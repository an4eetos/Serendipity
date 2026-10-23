// Package domain is the shared kernel: the business concepts every bounded
// context uses. Bounded contexts import it as "kernel".
package domain

// Kind classifies a business rule violation so that driving adapters can
// translate it for their transport.
type Kind int

const (
	// KindInvalid means the input breaks a business rule.
	KindInvalid Kind = iota + 1
	// KindNotFound means the thing does not exist or does not belong to the
	// caller.
	KindNotFound
	// KindConflict means the action clashes with the current state.
	KindConflict
	// KindUnavailable means something the action depends on is unavailable.
	KindUnavailable
	// KindForbidden means the caller may not do this.
	KindForbidden
)

// DomainError is a business rule violation with a constant, reader-facing
// message.
type DomainError interface {
	error
	Kind() Kind
}

// Error is the DomainError of every bounded context. Declare instances as
// package-level variables so that callers can match them with errors.Is.
type Error struct {
	kind Kind
	msg  string
}

func (e *Error) Error() string { return e.msg }

func (e *Error) Kind() Kind { return e.kind }

func Invalid(msg string) *Error { return &Error{kind: KindInvalid, msg: msg} }

func NotFound(msg string) *Error { return &Error{kind: KindNotFound, msg: msg} }

func Conflict(msg string) *Error { return &Error{kind: KindConflict, msg: msg} }

func Unavailable(msg string) *Error { return &Error{kind: KindUnavailable, msg: msg} }

func Forbidden(msg string) *Error { return &Error{kind: KindForbidden, msg: msg} }

// ErrNotFound is returned for anything that does not exist or is not the
// caller's to see.
var ErrNotFound = NotFound("not found")
