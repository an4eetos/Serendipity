package infra

import (
	"go.uber.org/fx"

	"github.com/an4eetos/serendipity/backend/infra/config"
	"github.com/an4eetos/serendipity/backend/internal/discussion/adapters/anthropic"
	"github.com/an4eetos/serendipity/backend/internal/discussion/adapters/handlers"
	"github.com/an4eetos/serendipity/backend/internal/discussion/adapters/postgres"
	"github.com/an4eetos/serendipity/backend/internal/discussion/domain"
	"github.com/an4eetos/serendipity/backend/internal/discussion/ports"
	"github.com/an4eetos/serendipity/backend/internal/discussion/usecase"
)

var Discussion = fx.Module("discussion",
	fx.Provide(
		fx.Annotate(postgres.NewPersonaRepository,
			fx.As(new(ports.PersonaRepository)), fx.As(new(ports.CharacterRepository)),
			fx.As(new(ports.PersonaModeration))),
		fx.Annotate(postgres.NewConversationRepository,
			fx.As(new(ports.ConversationRepository)), fx.As(new(ports.ConversationModeration))),
		fx.Annotate(newCharacterModel, fx.As(new(ports.CharacterModel))),
		usecase.NewListPersonas,
		usecase.NewGetThread,
		usecase.NewStartConversation,
		usecase.NewPostMessage,
		newCharacterLimit,
		usecase.NewListCharacters,
		usecase.NewCreateCharacter,
		usecase.NewUpdateCharacter,
		usecase.NewArchiveCharacter,
		usecase.NewUnarchiveCharacter,
		asRoutes(handlers.New),
		asRoutes(handlers.NewCharacters),
	),
)

func newCharacterModel(cfg config.Config) *anthropic.Character {
	return anthropic.NewCharacter(cfg.Anthropic.APIKey, cfg.Anthropic.Model)
}

// newCharacterLimit provides how many active characters a reader may have.
func newCharacterLimit(cfg config.Config) domain.CharacterLimit {
	return domain.CharacterLimit(cfg.Characters.ActiveLimit)
}
