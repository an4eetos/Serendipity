// Package converters maps moderation HTTP requests and responses to domain
// values. Reported content is embedded with the quote and discussion
// contexts' responses, in full: only admins see these.
package converters

import (
	"time"

	discussionconv "github.com/an4eetos/serendipity/backend/internal/discussion/adapters/converters"
	discussion "github.com/an4eetos/serendipity/backend/internal/discussion/domain"
	"github.com/an4eetos/serendipity/backend/internal/moderation/domain"
	quoteconv "github.com/an4eetos/serendipity/backend/internal/quote/adapters/converters"
)

// TargetRequest names a quote or a discussion.
type TargetRequest struct {
	TargetKind string `json:"target_kind"`
	TargetID   string `json:"target_id"`
}

// ToTargetRef returns the target the request names.
func (r TargetRequest) ToTargetRef() domain.TargetRef {
	return domain.TargetRef{Kind: domain.TargetKind(r.TargetKind), ID: r.TargetID}
}

// CreateReportRequest is a reader's report.
type CreateReportRequest struct {
	TargetRequest
	Reason string `json:"reason"`
	Note   string `json:"note"`
}

// ToReason returns the reason the reader picked.
func (r CreateReportRequest) ToReason() domain.Reason {
	return domain.Reason(r.Reason)
}

// DismissRequest names the target whose open reports an admin dismisses.
type DismissRequest struct {
	TargetRequest
}

// HideRequest is an admin's takedown of a target.
type HideRequest struct {
	TargetRequest
	Reason string `json:"reason"`
}

// ReportResponse is a report as its reporter sees it.
type ReportResponse struct {
	ID         string    `json:"id"`
	TargetKind string    `json:"target_kind"`
	TargetID   string    `json:"target_id"`
	Reason     string    `json:"reason"`
	Note       string    `json:"note"`
	Status     string    `json:"status"`
	CreatedAt  time.Time `json:"created_at"`
}

// ToReportResponse converts a report for its reporter.
func ToReportResponse(r domain.Report) ReportResponse {
	return ReportResponse{
		ID:         r.ID,
		TargetKind: string(r.Target.Kind),
		TargetID:   r.Target.ID,
		Reason:     string(r.Reason),
		Note:       r.Note,
		Status:     string(r.Status),
		CreatedAt:  r.CreatedAt,
	}
}

// ReportEntryResponse is one report on a reported target.
type ReportEntryResponse struct {
	ID         string    `json:"id"`
	ReporterID string    `json:"reporter_id"`
	Reason     string    `json:"reason"`
	Note       string    `json:"note"`
	CreatedAt  time.Time `json:"created_at"`
}

// ReportedTargetResponse is a reported target with its open reports. Quote
// is the quote, or the conversation's quote; both are null when the content
// was deleted.
type ReportedTargetResponse struct {
	TargetKind    string                               `json:"target_kind"`
	TargetID      string                               `json:"target_id"`
	OwnerID       string                               `json:"owner_id"`
	OwnerUsername string                               `json:"owner_username"`
	Quote         *quoteconv.QuoteResponse             `json:"quote"`
	Conversation  *discussionconv.ConversationResponse `json:"conversation"`
	Reports       []ReportEntryResponse                `json:"reports"`
}

// ToReportedTargetResponses converts reported targets for admins.
func ToReportedTargetResponses(ts []domain.ReportedTarget) []ReportedTargetResponse {
	out := make([]ReportedTargetResponse, len(ts))
	for i, t := range ts {
		reports := make([]ReportEntryResponse, len(t.Reports))
		for j, r := range t.Reports {
			reports[j] = ReportEntryResponse{
				ID:         r.ID,
				ReporterID: r.ReporterID,
				Reason:     string(r.Reason),
				Note:       r.Note,
				CreatedAt:  r.CreatedAt,
			}
		}
		out[i] = ReportedTargetResponse{
			TargetKind:    string(t.Ref.Kind),
			TargetID:      t.Ref.ID,
			OwnerID:       t.OwnerID,
			OwnerUsername: t.OwnerUsername,
			Reports:       reports,
		}
		out[i].Quote, out[i].Conversation = toContent(t.Content)
	}
	return out
}

// TakedownResponse is a takedown as admins see it.
type TakedownResponse struct {
	ID            string     `json:"id"`
	TargetKind    string     `json:"target_kind"`
	TargetID      string     `json:"target_id"`
	OwnerID       string     `json:"owner_id"`
	OwnerUsername string     `json:"owner_username"`
	Reason        string     `json:"reason"`
	HiddenBy      *string    `json:"hidden_by"`
	CreatedAt     time.Time  `json:"created_at"`
	RestoredAt    *time.Time `json:"restored_at"`
	RestoredBy    *string    `json:"restored_by"`
}

// ToTakedownResponse converts a takedown for admins.
func ToTakedownResponse(t domain.Takedown) TakedownResponse {
	return TakedownResponse{
		ID:            t.ID,
		TargetKind:    string(t.Target.Kind),
		TargetID:      t.Target.ID,
		OwnerID:       t.OwnerID,
		OwnerUsername: t.OwnerUsername,
		Reason:        t.Reason,
		HiddenBy:      t.HiddenBy,
		CreatedAt:     t.CreatedAt,
		RestoredAt:    t.RestoredAt,
		RestoredBy:    t.RestoredBy,
	}
}

// TakedownTargetResponse is an active takedown with the content it hides.
// Quote and conversation are null when the content was deleted.
type TakedownTargetResponse struct {
	TakedownResponse
	Quote        *quoteconv.QuoteResponse             `json:"quote"`
	Conversation *discussionconv.ConversationResponse `json:"conversation"`
}

// ToTakedownTargetResponses converts active takedowns, with their content, for admins.
func ToTakedownTargetResponses(vs []domain.TakedownView) []TakedownTargetResponse {
	out := make([]TakedownTargetResponse, len(vs))
	for i, v := range vs {
		out[i] = TakedownTargetResponse{TakedownResponse: ToTakedownResponse(v.Takedown)}
		out[i].Quote, out[i].Conversation = toContent(v.Content)
	}
	return out
}

// DismissResponse tells how many open reports were dismissed.
type DismissResponse struct {
	Dismissed int `json:"dismissed"`
}

// toContent embeds a target's quote and conversation; each is nil when
// absent.
func toContent(t *domain.Target) (*quoteconv.QuoteResponse, *discussionconv.ConversationResponse) {
	if t == nil {
		return nil, nil
	}
	var q *quoteconv.QuoteResponse
	if t.Quote != nil {
		resp := quoteconv.ToQuoteResponse(*t.Quote)
		q = &resp
	}
	var c *discussionconv.ConversationResponse
	if t.Conversation != nil {
		c = &discussionconv.ToConversationResponses([]discussion.Conversation{*t.Conversation})[0]
	}
	return q, c
}
