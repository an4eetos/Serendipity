package usecase

import (
	"context"

	discussion "github.com/an4eetos/serendipity/backend/internal/discussion/domain"
	discussionports "github.com/an4eetos/serendipity/backend/internal/discussion/ports"
	kernel "github.com/an4eetos/serendipity/backend/internal/kernel/domain"
	"github.com/an4eetos/serendipity/backend/internal/profile/domain"
	"github.com/an4eetos/serendipity/backend/internal/profile/ports"
	quoteports "github.com/an4eetos/serendipity/backend/internal/quote/ports"
)

// GetPublicProfile returns what a viewer may see of a reader's profile: their
// books, categories and public quotes with their discussions.
type GetPublicProfile struct {
	profiles      ports.ProfileRepository
	shelf         ports.ShelfRepository
	quotes        quoteports.QuoteRepository
	conversations discussionports.ConversationRepository
}

func NewGetPublicProfile(
	profiles ports.ProfileRepository,
	shelf ports.ShelfRepository,
	quotes quoteports.QuoteRepository,
	conversations discussionports.ConversationRepository,
) *GetPublicProfile {
	return &GetPublicProfile{profiles: profiles, shelf: shelf, quotes: quotes, conversations: conversations}
}

// Execute looks the profile up for viewerID, who is "" when anonymous.
func (uc *GetPublicProfile) Execute(ctx context.Context, viewerID, username string) (domain.PublicProfile, error) {
	p, err := uc.profiles.GetByUsername(ctx, domain.NormalizeUsername(username))
	if err != nil {
		return domain.PublicProfile{}, err
	}
	if !p.VisibleTo(viewerID) {
		return domain.PublicProfile{}, kernel.ErrNotFound
	}
	books, err := uc.shelf.Books(ctx, p.ID)
	if err != nil {
		return domain.PublicProfile{}, err
	}
	categories, err := uc.shelf.TopCategories(ctx, p.ID, domain.ShownCategories)
	if err != nil {
		return domain.PublicProfile{}, err
	}
	quotes, err := uc.quotes.ListPublic(ctx, p.ID, domain.ShownQuotes)
	if err != nil {
		return domain.PublicProfile{}, err
	}
	var convs []discussion.Conversation
	if len(quotes) > 0 {
		ids := make([]string, len(quotes))
		for i, q := range quotes {
			ids[i] = q.ID
		}
		if convs, err = uc.conversations.ListForQuotes(ctx, p.ID, ids); err != nil {
			return domain.PublicProfile{}, err
		}
	}
	return domain.NewPublicProfile(p, categories, books, quotes, convs), nil
}
