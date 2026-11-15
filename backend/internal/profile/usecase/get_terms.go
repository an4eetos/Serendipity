package usecase

import (
	"context"

	"github.com/an4eetos/serendipity/backend/internal/profile/domain"
)

// GetTerms returns the current version of the terms of service.
type GetTerms struct {
	terms domain.Terms
}

// NewGetTerms returns the GetTerms usecase for the current terms.
func NewGetTerms(terms domain.Terms) *GetTerms {
	return &GetTerms{terms: terms}
}

// Execute returns the current terms.
func (uc *GetTerms) Execute(_ context.Context) domain.Terms {
	return uc.terms
}
