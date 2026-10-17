package infra

import (
	"go.uber.org/fx"

	"github.com/an4eetos/serendipity/backend/internal/kindleimport/adapters/handlers"
	"github.com/an4eetos/serendipity/backend/internal/kindleimport/usecase"
)

// KindleImport reuses the catalog, library and quote repositories.
var KindleImport = fx.Module("kindleimport",
	fx.Provide(
		usecase.NewImportClippings,
		asRoutes(handlers.New),
	),
)
