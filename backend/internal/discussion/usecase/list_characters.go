package usecase

import (
	"context"

	"github.com/an4eetos/serendipity/backend/internal/discussion/domain"
	"github.com/an4eetos/serendipity/backend/internal/discussion/ports"
)

// ListCharacters lists a reader's own characters, in any state, with their
// cap.
type ListCharacters struct {
	chars ports.CharacterRepository
	limit domain.CharacterLimit
}

// NewListCharacters returns the ListCharacters usecase.
func NewListCharacters(chars ports.CharacterRepository, limit domain.CharacterLimit) *ListCharacters {
	return &ListCharacters{chars: chars, limit: limit}
}

// Execute lists ownerID's characters.
func (uc *ListCharacters) Execute(ctx context.Context, ownerID string) (domain.CharacterList, error) {
	cs, err := uc.chars.ListOwn(ctx, ownerID)
	if err != nil {
		return domain.CharacterList{}, err
	}
	return domain.NewCharacterList(cs, uc.limit), nil
}
