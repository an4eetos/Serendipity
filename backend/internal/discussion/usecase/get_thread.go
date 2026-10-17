package usecase

import (
	"context"

	"github.com/an4eetos/serendipity/backend/internal/discussion/domain"
	"github.com/an4eetos/serendipity/backend/internal/discussion/ports"
	quoteports "github.com/an4eetos/serendipity/backend/internal/quote/ports"
)

// GetThread returns one of the reader's quotes with its discussions.
type GetThread struct {
	quotes        quoteports.QuoteRepository
	conversations ports.ConversationRepository
}

func NewGetThread(quotes quoteports.QuoteRepository, conversations ports.ConversationRepository) *GetThread {
	return &GetThread{quotes: quotes, conversations: conversations}
}

func (uc *GetThread) Execute(ctx context.Context, userID, quoteID string) (domain.Thread, error) {
	q, err := uc.quotes.Get(ctx, userID, quoteID)
	if err != nil {
		return domain.Thread{}, err
	}
	convs, err := uc.conversations.ListForQuote(ctx, userID, q.ID)
	if err != nil {
		return domain.Thread{}, err
	}
	return domain.Thread{Quote: q, Conversations: convs}, nil
}
