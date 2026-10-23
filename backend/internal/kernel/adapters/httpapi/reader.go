package httpapi

import (
	"context"

	"github.com/go-chi/chi/v5"
)

type userIDKey struct{}

// WithUserID marks the request context as belonging to a signed-in reader.
func WithUserID(ctx context.Context, userID string) context.Context {
	return context.WithValue(ctx, userIDKey{}, userID)
}

// UserID returns the signed-in reader's id, or "" if anonymous.
func UserID(ctx context.Context) string {
	uid, _ := ctx.Value(userIDKey{}).(string)
	return uid
}

// Routes is implemented by each bounded context's HTTP handlers.
type Routes interface {
	// RegisterRoutes mounts the handlers: routes on public also serve
	// anonymous readers, routes on private require a signed-in reader who,
	// for anything but reads, has accepted the current terms of service.
	RegisterRoutes(public, private chi.Router)
}

// TermsRoutes is implemented by handlers with routes a signed-in reader may
// call before accepting the current terms of service.
type TermsRoutes interface {
	RegisterTermsRoutes(signedIn chi.Router)
}

// AdminRoutes is implemented by handlers with routes only admins may call.
type AdminRoutes interface {
	RegisterAdminRoutes(admin chi.Router)
}
