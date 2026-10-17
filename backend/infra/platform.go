package infra

import (
	"go.uber.org/fx"

	"github.com/an4eetos/serendipity/backend/infra/config"
	"github.com/an4eetos/serendipity/backend/infra/database"
	"github.com/an4eetos/serendipity/backend/infra/httpserver"
	kernelpg "github.com/an4eetos/serendipity/backend/internal/kernel/adapters/postgres"
	kernelports "github.com/an4eetos/serendipity/backend/internal/kernel/ports"
)

// Platform provides configuration, the database and the HTTP server.
var Platform = fx.Module("platform",
	fx.Provide(
		config.Load,
		database.NewPool,
		kernelpg.NewDB,
		fx.Annotate(kernelpg.NewTxManager, fx.As(new(kernelports.TxManager))),
		httpserver.NewRouter,
		httpserver.NewServer,
	),
)
