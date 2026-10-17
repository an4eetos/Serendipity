package usecase

import (
	"context"

	"github.com/an4eetos/serendipity/backend/internal/discussion/domain"
	"github.com/an4eetos/serendipity/backend/internal/discussion/ports"
)

// ListPersonas lists the characters a reader can talk to.
type ListPersonas struct {
	personas ports.PersonaRepository
}

func NewListPersonas(personas ports.PersonaRepository) *ListPersonas {
	return &ListPersonas{personas: personas}
}

func (uc *ListPersonas) Execute(ctx context.Context, userID string) ([]domain.Persona, error) {
	return uc.personas.List(ctx, userID)
}
