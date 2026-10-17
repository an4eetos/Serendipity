package infra

import (
	"go.uber.org/fx"

	"github.com/an4eetos/serendipity/backend/internal/quote/adapters/handlers"
	"github.com/an4eetos/serendipity/backend/internal/quote/adapters/postgres"
	"github.com/an4eetos/serendipity/backend/internal/quote/ports"
	"github.com/an4eetos/serendipity/backend/internal/quote/usecase"
)

var Quote = fx.Module("quote",
	fx.Provide(
		fx.Annotate(postgres.NewQuoteRepository, fx.As(new(ports.QuoteRepository))),
		usecase.NewCreateQuote,
		usecase.NewListQuotes,
		usecase.NewUpdateQuote,
		usecase.NewDeleteQuote,
		asRoutes(handlers.New),
	),
)
