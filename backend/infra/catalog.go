package infra

import (
	"go.uber.org/fx"

	"github.com/an4eetos/serendipity/backend/internal/catalog/adapters/handlers"
	"github.com/an4eetos/serendipity/backend/internal/catalog/adapters/openlibrary"
	"github.com/an4eetos/serendipity/backend/internal/catalog/adapters/postgres"
	"github.com/an4eetos/serendipity/backend/internal/catalog/ports"
	"github.com/an4eetos/serendipity/backend/internal/catalog/usecase"
)

var Catalog = fx.Module("catalog",
	fx.Provide(
		fx.Annotate(newOpenLibrary, fx.As(new(ports.Catalog))),
		fx.Annotate(postgres.NewWorkRepository, fx.As(new(ports.WorkRepository))),
		usecase.NewSearchWorks,
		asRoutes(handlers.New),
	),
)

func newOpenLibrary() *openlibrary.Client {
	return openlibrary.New(openlibrary.DefaultBaseURL)
}
