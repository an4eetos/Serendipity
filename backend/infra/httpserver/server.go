// Package httpserver serves the API over HTTP.
package httpserver

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"go.uber.org/fx"

	"github.com/an4eetos/serendipity/backend/infra/config"
	"github.com/an4eetos/serendipity/backend/internal/identity/adapters/middleware"
	"github.com/an4eetos/serendipity/backend/internal/kernel/adapters/httpapi"
)

type RouterParams struct {
	fx.In

	Config config.Config
	Auth   *middleware.Auth
	Routes []httpapi.Routes `group:"routes"`
}

// NewRouter mounts every bounded context's routes behind the shared
// middleware.
func NewRouter(p RouterParams) http.Handler {
	r := chi.NewRouter()
	r.Use(chimw.RequestID, chimw.RealIP, chimw.Logger, chimw.Recoverer)
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins: p.Config.HTTP.AllowedOrigins,
		AllowedMethods: []string{"GET", "POST", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders: []string{"Authorization", "Content-Type"},
		MaxAge:         300,
	}))

	r.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) { w.Write([]byte("ok")) })

	public, private := r.With(p.Auth.Optional), r.With(p.Auth.Require)
	for _, routes := range p.Routes {
		routes.RegisterRoutes(public, private)
	}
	return r
}

// NewServer serves handler while the app runs, and shuts the app down if it
// stops serving on its own.
func NewServer(lc fx.Lifecycle, shutdowner fx.Shutdowner, cfg config.Config, handler http.Handler) *http.Server {
	srv := &http.Server{
		Addr:              cfg.HTTP.Addr,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
	}
	lc.Append(fx.Hook{
		OnStart: func(context.Context) error {
			ln, err := net.Listen("tcp", srv.Addr)
			if err != nil {
				return err
			}
			slog.Info("listening", "addr", srv.Addr, "model", cfg.Anthropic.Model)
			go func() {
				if err := srv.Serve(ln); !errors.Is(err, http.ErrServerClosed) {
					slog.Error("server stopped", "err", err)
					_ = shutdowner.Shutdown(fx.ExitCode(1))
				}
			}()
			return nil
		},
		OnStop: srv.Shutdown,
	})
	return srv
}
