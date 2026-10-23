package infra

import (
	"go.uber.org/fx"

	"github.com/an4eetos/serendipity/backend/infra/config"
	"github.com/an4eetos/serendipity/backend/internal/quote/adapters/handlers"
	"github.com/an4eetos/serendipity/backend/internal/quote/adapters/postgres"
	"github.com/an4eetos/serendipity/backend/internal/quote/domain"
	"github.com/an4eetos/serendipity/backend/internal/quote/ports"
	"github.com/an4eetos/serendipity/backend/internal/quote/usecase"
)

var Quote = fx.Module("quote",
	fx.Provide(
		fx.Annotate(postgres.NewQuoteRepository,
			fx.As(new(ports.QuoteRepository)), fx.As(new(ports.QuoteModeration))),
		fx.Annotate(postgres.NewCoverageRepository, fx.As(new(ports.CoverageRepository))),
		newCoverageCap,
		usecase.NewCreateQuote,
		usecase.NewListQuotes,
		usecase.NewUpdateQuote,
		usecase.NewDeleteQuote,
		asRoutes(handlers.New),
	),
)

// newCoverageCap provides the default public-coverage cap per work.
func newCoverageCap(cfg config.Config) domain.CoverageCap {
	return domain.CoverageCap(cfg.Coverage.PublicWordCap)
}
