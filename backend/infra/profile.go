package infra

import (
	"go.uber.org/fx"

	"github.com/an4eetos/serendipity/backend/internal/profile/adapters/handlers"
	"github.com/an4eetos/serendipity/backend/internal/profile/adapters/postgres"
	"github.com/an4eetos/serendipity/backend/internal/profile/ports"
	"github.com/an4eetos/serendipity/backend/internal/profile/usecase"
)

var Profile = fx.Module("profile",
	fx.Provide(
		fx.Annotate(postgres.NewProfileRepository, fx.As(new(ports.ProfileRepository))),
		fx.Annotate(postgres.NewShelfRepository, fx.As(new(ports.ShelfRepository))),
		usecase.NewGetMe,
		usecase.NewUpdateMe,
		usecase.NewGetPublicProfile,
		asRoutes(handlers.New),
	),
)
