package usecase

import (
	"context"

	discussionports "github.com/an4eetos/serendipity/backend/internal/discussion/ports"
	"github.com/an4eetos/serendipity/backend/internal/moderation/domain"
	"github.com/an4eetos/serendipity/backend/internal/moderation/ports"
	quoteports "github.com/an4eetos/serendipity/backend/internal/quote/ports"
)

// ListTakedowns lists the active takedowns with the content they hide, for
// admins to restore.
type ListTakedowns struct {
	takedowns ports.TakedownRepository
	targets   targets
}

// NewListTakedowns returns the ListTakedowns usecase.
func NewListTakedowns(
	takedowns ports.TakedownRepository,
	quotes quoteports.QuoteModeration,
	conversations discussionports.ConversationModeration,
) *ListTakedowns {
	return &ListTakedowns{takedowns: takedowns, targets: targets{quotes: quotes, conversations: conversations}}
}

// Execute lists the newest domain.MaxListed active takedowns.
func (uc *ListTakedowns) Execute(ctx context.Context) ([]domain.TakedownView, error) {
	tds, err := uc.takedowns.ListActive(ctx, domain.MaxListed)
	if err != nil {
		return nil, err
	}
	refs := make([]domain.TargetRef, len(tds))
	for i, td := range tds {
		refs[i] = td.Target
	}
	content, err := uc.targets.loadAll(ctx, refs)
	if err != nil {
		return nil, err
	}
	views := make([]domain.TakedownView, len(tds))
	for i, td := range tds {
		views[i] = domain.TakedownView{Takedown: td}
		if t, ok := content[td.Target]; ok {
			views[i].Content = &t
		}
	}
	return views, nil
}
