package usecase

import (
	"context"

	"github.com/an4eetos/serendipity/backend/internal/discussion/domain"
	"github.com/an4eetos/serendipity/backend/internal/discussion/ports"
	kernelports "github.com/an4eetos/serendipity/backend/internal/kernel/ports"
	quoteports "github.com/an4eetos/serendipity/backend/internal/quote/ports"
)

// StartConversation starts a discussion of the reader's reaction to a quote
// and streams the character's first reply.
type StartConversation struct {
	replier
	quotes   quoteports.QuoteRepository
	personas ports.PersonaRepository
}

func NewStartConversation(
	txm kernelports.TxManager,
	quotes quoteports.QuoteRepository,
	personas ports.PersonaRepository,
	conversations ports.ConversationRepository,
	model ports.CharacterModel,
) *StartConversation {
	return &StartConversation{
		replier:  replier{txm: txm, conversations: conversations, model: model},
		quotes:   quotes,
		personas: personas,
	}
}

func (uc *StartConversation) Execute(ctx context.Context, userID, quoteID, personaID string, out ports.ReplyStream) (domain.Message, error) {
	q, err := uc.quotes.Get(ctx, userID, quoteID)
	if err != nil {
		return domain.Message{}, err
	}
	if err := domain.CheckDiscussable(q); err != nil {
		return domain.Message{}, err
	}
	persona, err := uc.personas.Get(ctx, userID, personaID)
	if err != nil {
		return domain.Message{}, err
	}
	var convID string
	err = uc.txm.WithinTx(ctx, func(ctx context.Context) error {
		var err error
		convID, err = uc.conversations.Create(ctx, userID, q.ID, persona.ID)
		return err
	})
	if err != nil {
		return domain.Message{}, err
	}

	prompt := domain.NewCharacterPrompt(persona, domain.PassageOf(q), nil)
	m, err := uc.answer(ctx, convID, prompt, out)
	if err != nil {
		// Don't leave an empty conversation behind.
		return domain.Message{}, uc.undo(ctx, err, func(ctx context.Context) error {
			return uc.conversations.Delete(ctx, userID, convID)
		})
	}
	return m, nil
}
