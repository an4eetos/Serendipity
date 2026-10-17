package infra

import (
	"go.uber.org/fx"

	"github.com/an4eetos/serendipity/backend/infra/config"
	"github.com/an4eetos/serendipity/backend/internal/library/adapters/handlers"
	"github.com/an4eetos/serendipity/backend/internal/library/adapters/postgres"
	"github.com/an4eetos/serendipity/backend/internal/library/adapters/supabase"
	"github.com/an4eetos/serendipity/backend/internal/library/ports"
	"github.com/an4eetos/serendipity/backend/internal/library/usecase"
)

var Library = fx.Module("library",
	fx.Provide(
		fx.Annotate(postgres.NewLibraryRepository, fx.As(new(ports.LibraryRepository))),
		fx.Annotate(newFileStorage, fx.As(new(ports.FileStorage))),
		usecase.NewAddBook,
		usecase.NewListBooks,
		usecase.NewGetBook,
		usecase.NewUpdateBook,
		usecase.NewGetFileLink,
		asRoutes(handlers.New),
	),
)

func newFileStorage(cfg config.Config) *supabase.Storage {
	return supabase.NewStorage(cfg.Supabase.URL, cfg.Supabase.ServiceRoleKey)
}
