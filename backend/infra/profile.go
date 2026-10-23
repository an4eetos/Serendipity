package infra

import (
	"go.uber.org/fx"

	"github.com/an4eetos/serendipity/backend/infra/config"
	"github.com/an4eetos/serendipity/backend/internal/profile/adapters/handlers"
	"github.com/an4eetos/serendipity/backend/internal/profile/adapters/middleware"
	"github.com/an4eetos/serendipity/backend/internal/profile/adapters/postgres"
	"github.com/an4eetos/serendipity/backend/internal/profile/domain"
	"github.com/an4eetos/serendipity/backend/internal/profile/ports"
	"github.com/an4eetos/serendipity/backend/internal/profile/usecase"
)

var Profile = fx.Module("profile",
	fx.Provide(
		fx.Annotate(postgres.NewProfileRepository, fx.As(new(ports.ProfileRepository))),
		fx.Annotate(postgres.NewShelfRepository, fx.As(new(ports.ShelfRepository))),
		newTerms,
		usecase.NewGetMe,
		usecase.NewUpdateMe,
		usecase.NewGetPublicProfile,
		usecase.NewGetTerms,
		usecase.NewAcceptTerms,
		usecase.NewCheckTerms,
		middleware.NewTerms,
		asRoutes(handlers.New),
	),
)

// newTerms provides the current terms of service.
func newTerms(cfg config.Config) domain.Terms {
	return domain.Terms{Version: cfg.Terms.Version}
}
