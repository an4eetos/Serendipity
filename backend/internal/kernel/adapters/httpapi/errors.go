// Package httpapi is the HTTP plumbing every bounded context's handlers
// build on: JSON bodies, error translation, server-sent events and the
// signed-in reader.
package httpapi

import (
	"errors"
	"log/slog"
	"net/http"

	kernel "github.com/an4eetos/serendipity/backend/internal/kernel/domain"
)

// Error is a transport-level failure, such as a malformed request body.
type Error struct {
	Status int
	Msg    string
}

func (e *Error) Error() string { return e.Msg }

func BadRequest(msg string) *Error {
	return &Error{Status: http.StatusBadRequest, Msg: msg}
}

type errorBody struct {
	Error string `json:"error"`
}

// WriteError writes err as a JSON error response. Transport and domain
// errors keep their message; anything else is logged and reported as an
// internal error.
func WriteError(w http.ResponseWriter, r *http.Request, err error) {
	var te *Error
	var de kernel.DomainError
	switch {
	case errors.As(err, &te):
		WriteJSON(w, te.Status, errorBody{te.Msg})
	case errors.As(err, &de):
		WriteJSON(w, StatusOf(de), errorBody{de.Error()})
	default:
		// A client that went away is not a server error.
		if r.Context().Err() == nil {
			slog.Error("request failed", "path", r.URL.Path, "err", err)
		}
		WriteJSON(w, http.StatusInternalServerError, errorBody{"internal error"})
	}
}

// StatusOf translates a domain error into an HTTP status.
func StatusOf(err kernel.DomainError) int {
	switch err.Kind() {
	case kernel.KindInvalid:
		return http.StatusBadRequest
	case kernel.KindNotFound:
		return http.StatusNotFound
	case kernel.KindConflict:
		return http.StatusConflict
	case kernel.KindUnavailable:
		return http.StatusBadGateway
	}
	return http.StatusInternalServerError
}
