package infra

import (
	"context"

	"go.uber.org/fx"

	"github.com/an4eetos/serendipity/backend/infra/config"
	"github.com/an4eetos/serendipity/backend/internal/identity/adapters/middleware"
	"github.com/an4eetos/serendipity/backend/internal/identity/adapters/supabase"
	"github.com/an4eetos/serendipity/backend/internal/identity/ports"
)

var Identity = fx.Module("identity",
	fx.Provide(
		fx.Annotate(newTokenVerifier, fx.As(new(ports.TokenVerifier))),
		middleware.NewAuth,
	),
)

// newTokenVerifier keeps the JWKS refreshed while the app runs.
func newTokenVerifier(lc fx.Lifecycle, cfg config.Config) *supabase.Verifier {
	ctx, cancel := context.WithCancel(context.Background())
	lc.Append(fx.StopHook(cancel))
	return supabase.NewVerifier(ctx, cfg.Supabase.URL, cfg.Supabase.JWTSecret)
}
