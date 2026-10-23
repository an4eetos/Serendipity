// Package usecase holds the moderation context's business actions.
package usecase

import (
	"context"

	discussion "github.com/an4eetos/serendipity/backend/internal/discussion/domain"
	discussionports "github.com/an4eetos/serendipity/backend/internal/discussion/ports"
	kernel "github.com/an4eetos/serendipity/backend/internal/kernel/domain"
	"github.com/an4eetos/serendipity/backend/internal/moderation/domain"
	quote "github.com/an4eetos/serendipity/backend/internal/quote/domain"
	quoteports "github.com/an4eetos/serendipity/backend/internal/quote/ports"
)

// targets reads and changes reportable content through the quote and
// discussion contexts.
type targets struct {
	quotes        quoteports.QuoteModeration
	conversations discussionports.ConversationModeration
}

// load returns the target ref names, or kernel.ErrNotFound when it doesn't
// exist. Call it before starting a transaction: ref comes from a reader.
func (t targets) load(ctx context.Context, ref domain.TargetRef) (domain.Target, error) {
	switch ref.Kind {
	case domain.TargetQuote:
		qs, err := t.quotes.ListAny(ctx, []string{ref.ID})
		if err != nil {
			return domain.Target{}, err
		}
		if len(qs) == 0 {
			return domain.Target{}, kernel.ErrNotFound
		}
		return domain.QuoteTarget(qs[0]), nil
	case domain.TargetConversation:
		cs, err := t.conversations.ListAny(ctx, []string{ref.ID})
		if err != nil {
			return domain.Target{}, err
		}
		if len(cs) == 0 {
			return domain.Target{}, kernel.ErrNotFound
		}
		qs, err := t.quotes.ListAny(ctx, []string{cs[0].QuoteID})
		if err != nil {
			return domain.Target{}, err
		}
		if len(qs) == 0 {
			return domain.Target{}, kernel.ErrNotFound
		}
		return domain.ConversationTarget(cs[0], qs[0]), nil
	default:
		return domain.Target{}, domain.ErrInvalidTarget
	}
}

// loadAll returns the targets refs name that still exist, by ref. It reads
// each context at most once: one ConversationModeration.ListAny, then one
// QuoteModeration.ListAny for the quote refs plus the conversations' quotes.
// Refs of other kinds are left out.
func (t targets) loadAll(ctx context.Context, refs []domain.TargetRef) (map[domain.TargetRef]domain.Target, error) {
	out := make(map[domain.TargetRef]domain.Target, len(refs))
	var quoteIDs, conversationIDs []string
	for _, ref := range refs {
		switch ref.Kind {
		case domain.TargetQuote:
			quoteIDs = append(quoteIDs, ref.ID)
		case domain.TargetConversation:
			conversationIDs = append(conversationIDs, ref.ID)
		default:
			// A kind this version can't show is listed without content.
		}
	}

	var convs []discussion.Conversation
	if len(conversationIDs) > 0 {
		var err error
		if convs, err = t.conversations.ListAny(ctx, conversationIDs); err != nil {
			return nil, err
		}
		for _, c := range convs {
			quoteIDs = append(quoteIDs, c.QuoteID)
		}
	}
	if len(quoteIDs) == 0 {
		return out, nil
	}
	qs, err := t.quotes.ListAny(ctx, unique(quoteIDs))
	if err != nil {
		return nil, err
	}

	byID := make(map[string]quote.Quote, len(qs))
	for _, q := range qs {
		byID[q.ID] = q
	}
	for _, ref := range refs {
		if q, ok := byID[ref.ID]; ok && ref.Kind == domain.TargetQuote {
			out[ref] = domain.QuoteTarget(q)
		}
	}
	for _, c := range convs {
		if q, ok := byID[c.QuoteID]; ok {
			out[domain.TargetRef{Kind: domain.TargetConversation, ID: c.ID}] = domain.ConversationTarget(c, q)
		}
	}
	return out, nil
}

// remove hides the target from every public read. It must run inside the
// TxManager.
func (t targets) remove(ctx context.Context, ref domain.TargetRef, reason string) error {
	switch ref.Kind {
	case domain.TargetQuote:
		return t.quotes.Remove(ctx, ref.ID, reason)
	case domain.TargetConversation:
		return t.conversations.Remove(ctx, ref.ID, reason)
	default:
		return domain.ErrInvalidTarget
	}
}

// restore puts the target back in public reads. A restored quote counts
// toward its work's coverage again, so it is checked against wordCap;
// conversations don't count. It must run inside the TxManager.
func (t targets) restore(ctx context.Context, ref domain.TargetRef, wordCap quote.CoverageCap) error {
	switch ref.Kind {
	case domain.TargetQuote:
		return t.quotes.Restore(ctx, ref.ID, wordCap)
	case domain.TargetConversation:
		return t.conversations.Restore(ctx, ref.ID)
	default:
		return domain.ErrInvalidTarget
	}
}

// unique returns ids without repeats, in their first order.
func unique(ids []string) []string {
	seen := make(map[string]bool, len(ids))
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		if !seen[id] {
			seen[id] = true
			out = append(out, id)
		}
	}
	return out
}
