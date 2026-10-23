// Package converters maps quote HTTP requests and responses to domain
// values. Other contexts use QuoteResponse to embed quotes in theirs.
package converters

import (
	"time"

	catalogconv "github.com/an4eetos/serendipity/backend/internal/catalog/adapters/converters"
	"github.com/an4eetos/serendipity/backend/internal/quote/domain"
)

type CreateRequest struct {
	LibraryItemID string   `json:"library_item_id"`
	Text          string   `json:"text"`
	Location      string   `json:"location"`
	Anchor        *string  `json:"anchor"`
	Position      *float64 `json:"position"`
	Reaction      string   `json:"reaction"`
}

func (r CreateRequest) ToDraft() domain.Draft {
	return domain.Draft{
		Text:     r.Text,
		Location: r.Location,
		Anchor:   r.Anchor,
		Position: r.Position,
		Reaction: r.Reaction,
	}
}

// PatchRequest has no passage_hidden: the publishing rules decide it, and
// decoding rejects the unknown field.
type PatchRequest struct {
	Reaction   *string `json:"reaction"`
	Visibility *string `json:"visibility"`
}

func (r PatchRequest) ToPatch() domain.Patch {
	p := domain.Patch{Reaction: r.Reaction}
	if r.Visibility != nil {
		v := domain.Visibility(*r.Visibility)
		p.Visibility = &v
	}
	return p
}

type QuoteResponse struct {
	ID            string                   `json:"id"`
	WorkID        string                   `json:"work_id"`
	LibraryItemID *string                  `json:"library_item_id"`
	Text          string                   `json:"text"`
	Location      string                   `json:"location"`
	Anchor        *string                  `json:"anchor"`
	Position      *float64                 `json:"position"`
	Reaction      string                   `json:"reaction"`
	Source        string                   `json:"source"`
	Visibility    string                   `json:"visibility"`
	PassageHidden bool                     `json:"passage_hidden"`
	RemovedAt     *time.Time               `json:"removed_at"`
	RemovedReason string                   `json:"removed_reason"`
	CreatedAt     time.Time                `json:"created_at"`
	UpdatedAt     time.Time                `json:"updated_at"`
	Work          catalogconv.WorkResponse `json:"work"`
}

func ToQuoteResponse(q domain.Quote) QuoteResponse {
	return QuoteResponse{
		ID:            q.ID,
		WorkID:        q.WorkID,
		LibraryItemID: q.LibraryItemID,
		Text:          q.Text,
		Location:      q.Location,
		Anchor:        q.Anchor,
		Position:      q.Position,
		Reaction:      q.Reaction,
		Source:        string(q.Source),
		Visibility:    string(q.Visibility),
		PassageHidden: q.PassageHidden,
		RemovedAt:     q.RemovedAt,
		RemovedReason: q.RemovedReason,
		CreatedAt:     q.CreatedAt,
		UpdatedAt:     q.UpdatedAt,
		Work:          catalogconv.ToWorkResponse(q.Work),
	}
}

func ToQuoteResponses(qs []domain.Quote) []QuoteResponse {
	out := make([]QuoteResponse, len(qs))
	for i, q := range qs {
		out[i] = ToQuoteResponse(q)
	}
	return out
}
