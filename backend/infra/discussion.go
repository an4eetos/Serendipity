package infra

import (
	"go.uber.org/fx"

	"github.com/an4eetos/serendipity/backend/infra/config"
	"github.com/an4eetos/serendipity/backend/internal/discussion/adapters/anthropic"
	"github.com/an4eetos/serendipity/backend/internal/discussion/adapters/handlers"
	"github.com/an4eetos/serendipity/backend/internal/discussion/adapters/postgres"
	"github.com/an4eetos/serendipity/backend/internal/discussion/ports"
	"github.com/an4eetos/serendipity/backend/internal/discussion/usecase"
)

var Discussion = fx.Module("discussion",
	fx.Provide(
		fx.Annotate(postgres.NewPersonaRepository, fx.As(new(ports.PersonaRepository))),
		fx.Annotate(postgres.NewConversationRepository, fx.As(new(ports.ConversationRepository))),
		fx.Annotate(newCharacterModel, fx.As(new(ports.CharacterModel))),
		usecase.NewListPersonas,
		usecase.NewGetThread,
		usecase.NewStartConversation,
		usecase.NewPostMessage,
		asRoutes(handlers.New),
	),
)

func newCharacterModel(cfg config.Config) *anthropic.Character {
	return anthropic.NewCharacter(cfg.Anthropic.APIKey, cfg.Anthropic.Model)
}
