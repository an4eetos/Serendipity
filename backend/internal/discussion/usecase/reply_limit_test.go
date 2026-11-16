package usecase

import (
	"errors"
	"reflect"
	"testing"

	"github.com/an4eetos/serendipity/backend/internal/discussion/domain"
	"github.com/an4eetos/serendipity/backend/internal/kernel/ports/portstest"
	quote "github.com/an4eetos/serendipity/backend/internal/quote/domain"
)

func TestStartConversationReplyLimit(t *testing.T) {
	quotes := &fakeQuotes{quotes: map[string]quote.Quote{reacted.ID: reacted}}
	start := func(log *fakeReplyLog, model *fakeModel) (*fakeConversations, *fakeStream, error) {
		convs, out := &fakeConversations{}, &fakeStream{}
		uc := NewStartConversation(&portstest.TxManager{}, quotes, &fakePersonas{persona: diogenes}, convs, model, log, 3)
		_, err := uc.Execute(t.Context(), "u1", reacted.ID, diogenes.ID, out)
		return convs, out, err
	}

	t.Run("counts the reply in the transaction that creates the conversation", func(t *testing.T) {
		log := &fakeReplyLog{recent: 2}
		if _, _, err := start(log, &fakeModel{chunks: []string{"Hi."}}); err != nil {
			t.Fatal(err)
		}
		want := []string{"lock", "count 1h0m0s", "add 1h0m0s"}
		if !reflect.DeepEqual(log.calls, want) || len(log.outsideTx) > 0 {
			t.Errorf("calls %v (outside a transaction %v), want %v in one", log.calls, log.outsideTx, want)
		}
	})

	t.Run("refuses at the limit before anything is stored or paid for", func(t *testing.T) {
		log, model := &fakeReplyLog{recent: 3}, &fakeModel{chunks: []string{"Hi."}}
		convs, out, err := start(log, model)
		if !errors.Is(err, domain.ErrReplyLimit) {
			t.Fatalf("err = %v, want ErrReplyLimit", err)
		}
		if len(convs.created) > 0 || model.calls > 0 || out.started != "" || log.added > 0 {
			t.Errorf("created %v, model calls %d, started %q, added %d; want nothing",
				convs.created, model.calls, out.started, log.added)
		}
	})

	t.Run("a failed reply still counts", func(t *testing.T) {
		log := &fakeReplyLog{}
		_, _, err := start(log, &fakeModel{err: errors.New("overloaded")})
		if !errors.Is(err, domain.ErrCharacterUnavailable) {
			t.Fatalf("err = %v, want ErrCharacterUnavailable", err)
		}
		if log.added != 1 {
			t.Errorf("added %d, want 1: the model may have been paid for", log.added)
		}
	})
}

func TestPostMessageReplyLimit(t *testing.T) {
	answered := domain.Conversation{
		ID: "c1", QuoteID: reacted.ID, Persona: diogenes,
		Messages: []domain.Message{{ID: 1, Role: domain.RoleAssistant, Content: "Do you, though?"}},
	}
	quotes := &fakeQuotes{quotes: map[string]quote.Quote{reacted.ID: reacted}}
	post := func(log *fakeReplyLog, model *fakeModel) (*fakeConversations, error) {
		convs := &fakeConversations{conv: answered}
		uc := NewPostMessage(&portstest.TxManager{}, quotes, convs, model, log, 3)
		_, err := uc.Execute(t.Context(), "u1", "c1", "Sometimes.", &fakeStream{})
		return convs, err
	}

	t.Run("counts the reply with the reader's message", func(t *testing.T) {
		log := &fakeReplyLog{recent: 2}
		if _, err := post(log, &fakeModel{chunks: []string{"Then act on it."}}); err != nil {
			t.Fatal(err)
		}
		if log.added != 1 || len(log.outsideTx) > 0 {
			t.Errorf("added %d, calls outside a transaction %v", log.added, log.outsideTx)
		}
	})

	t.Run("refuses at the limit before the message is stored", func(t *testing.T) {
		log, model := &fakeReplyLog{recent: 3}, &fakeModel{chunks: []string{"Then act on it."}}
		convs, err := post(log, model)
		if !errors.Is(err, domain.ErrReplyLimit) {
			t.Fatalf("err = %v, want ErrReplyLimit", err)
		}
		if len(convs.appended) > 0 || model.calls > 0 {
			t.Errorf("appended %v, model calls %d; want nothing", convs.appended, model.calls)
		}
	})
}
