// Package infra wires the application: one Fx module per bounded context,
// plus the platform they run on.
package infra

import (
	"log/slog"

	"go.uber.org/fx"
	"go.uber.org/fx/fxevent"

	"github.com/an4eetos/serendipity/backend/internal/kernel/adapters/httpapi"
)

// App is the whole application.
var App = fx.Options(
	fx.WithLogger(newFxLogger),
	Platform,
	Identity,
	Catalog,
	Library,
	Quote,
	Discussion,
	KindleImport,
	Profile,
)

// newFxLogger logs Fx's own lifecycle at debug level and its errors as
// errors.
func newFxLogger() fxevent.Logger {
	l := &fxevent.SlogLogger{Logger: slog.Default()}
	l.UseLogLevel(slog.LevelDebug)
	return l
}

// asRoutes provides a bounded context's HTTP handlers to the router.
func asRoutes(constructor any) any {
	return fx.Annotate(constructor, fx.As(new(httpapi.Routes)), fx.ResultTags(`group:"routes"`))
}
