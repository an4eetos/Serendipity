package usecase

import (
	"context"

	"github.com/an4eetos/serendipity/backend/internal/discussion/domain"
	"github.com/an4eetos/serendipity/backend/internal/discussion/ports"
	kernelports "github.com/an4eetos/serendipity/backend/internal/kernel/ports"
	quoteports "github.com/an4eetos/serendipity/backend/internal/quote/ports"
)

// PostMessage adds the reader's reply to a conversation and streams the
// character's answer.
type PostMessage struct {
	replier
	quotes quoteports.QuoteRepository
}

func NewPostMessage(
	txm kernelports.TxManager,
	quotes quoteports.QuoteRepository,
	conversations ports.ConversationRepository,
	model ports.CharacterModel,
) *PostMessage {
	return &PostMessage{
		replier: replier{txm: txm, conversations: conversations, model: model},
		quotes:  quotes,
	}
}

func (uc *PostMessage) Execute(ctx context.Context, userID, conversationID, content string, out ports.ReplyStream) (domain.Message, error) {
	content, err := domain.NewReaderMessage(content)
	if err != nil {
		return domain.Message{}, err
	}
	conv, err := uc.conversations.Get(ctx, userID, conversationID)
	if err != nil {
		return domain.Message{}, err
	}
	if err := conv.CheckReaderTurn(); err != nil {
		return domain.Message{}, err
	}
	q, err := uc.quotes.Get(ctx, userID, conv.QuoteID)
	if err != nil {
		return domain.Message{}, err
	}
	if err := domain.CheckDiscussable(q); err != nil {
		return domain.Message{}, err
	}
	var userMsg domain.Message
	err = uc.txm.WithinTx(ctx, func(ctx context.Context) error {
		var err error
		userMsg, err = uc.conversations.AppendMessage(ctx, conv.ID, domain.RoleUser, content)
		return err
	})
	if err != nil {
		return domain.Message{}, err
	}

	history := append(conv.Messages, userMsg)
	prompt := domain.NewCharacterPrompt(conv.Persona, domain.PassageOf(q), history)
	m, err := uc.answer(ctx, conv.ID, prompt, out)
	if err != nil {
		// Drop the unanswered message so the history keeps alternating and
		// the reader can simply send it again.
		return domain.Message{}, uc.undo(ctx, err, func(ctx context.Context) error {
			return uc.conversations.DeleteMessage(ctx, conv.ID, userMsg.ID)
		})
	}
	return m, nil
}
