// Package usecase holds the discussion context's business actions.
package usecase

import (
	"context"
	"errors"

	"github.com/an4eetos/serendipity/backend/internal/discussion/domain"
	"github.com/an4eetos/serendipity/backend/internal/discussion/ports"
	kernelports "github.com/an4eetos/serendipity/backend/internal/kernel/ports"
)

// CleanupError reports that undoing a failed reply failed too, leaving an
// empty conversation or an unanswered message behind.
type CleanupError struct {
	Err error
}

func (e *CleanupError) Error() string { return "clean up after a failed reply: " + e.Err.Error() }

func (e *CleanupError) Unwrap() error { return e.Err }

// replier has a character answer in a conversation.
type replier struct {
	txm           kernelports.TxManager
	conversations ports.ConversationRepository
	model         ports.CharacterModel
}

// answer streams the character's reply to out and stores it. The reader may
// leave before the reply is stored, so storing does not depend on ctx.
func (r replier) answer(ctx context.Context, conversationID string, prompt domain.CharacterPrompt, out ports.ReplyStream) (domain.Message, error) {
	if err := out.Started(conversationID); err != nil {
		return domain.Message{}, err
	}
	reply, err := r.model.Reply(ctx, prompt, out.Text)
	if err != nil {
		if errors.Is(err, domain.ErrRefused) {
			return domain.Message{}, err
		}
		return domain.Message{}, errors.Join(domain.ErrCharacterUnavailable, err)
	}
	var m domain.Message
	err = r.txm.WithinTx(context.WithoutCancel(ctx), func(ctx context.Context) error {
		var err error
		m, err = r.conversations.AppendMessage(ctx, conversationID, domain.RoleAssistant, reply)
		return err
	})
	if err != nil {
		return domain.Message{}, errors.Join(domain.ErrReplyNotSaved, err)
	}
	return m, nil
}

// undo reverts what was stored for a reply that failed with err, and
// returns err with any failure to do so.
func (r replier) undo(ctx context.Context, err error, revert func(ctx context.Context) error) error {
	if cerr := r.txm.WithinTx(context.WithoutCancel(ctx), revert); cerr != nil {
		return errors.Join(err, &CleanupError{Err: cerr})
	}
	return err
}
