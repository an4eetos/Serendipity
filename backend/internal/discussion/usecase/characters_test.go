package usecase

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/an4eetos/serendipity/backend/internal/discussion/domain"
	kernel "github.com/an4eetos/serendipity/backend/internal/kernel/domain"
	"github.com/an4eetos/serendipity/backend/internal/kernel/ports/portstest"
	quote "github.com/an4eetos/serendipity/backend/internal/quote/domain"
)

var archivedAt = time.Date(2026, 11, 16, 12, 0, 0, 0, time.UTC)

// fakeCharacters implements ports.CharacterRepository on a map of one
// owner's characters. It logs every call, marking those made inside a unit
// of work.
type fakeCharacters struct {
	byID   map[string]domain.Persona
	active int // what CountActive returns
	calls  []string
	nextID int
}

func (f *fakeCharacters) add(ctx context.Context, format string, args ...any) {
	call := fmt.Sprintf(format, args...)
	if portstest.InTx(ctx) {
		call += " (tx)"
	}
	f.calls = append(f.calls, call)
}

func (f *fakeCharacters) ListOwn(ctx context.Context, ownerID string) ([]domain.Persona, error) {
	f.add(ctx, "ListOwn %s", ownerID)
	out := []domain.Persona{}
	for _, p := range f.byID {
		if p.OwnerID == ownerID {
			out = append(out, p)
		}
	}
	return out, nil
}

func (f *fakeCharacters) get(ownerID, id string) (domain.Persona, error) {
	p, ok := f.byID[id]
	if !ok || p.OwnerID != ownerID {
		return domain.Persona{}, kernel.ErrNotFound
	}
	return p, nil
}

func (f *fakeCharacters) GetOwn(ctx context.Context, ownerID, id string) (domain.Persona, error) {
	f.add(ctx, "GetOwn %s", id)
	return f.get(ownerID, id)
}

func (f *fakeCharacters) GetOwnForUpdate(ctx context.Context, ownerID, id string) (domain.Persona, error) {
	f.add(ctx, "GetOwnForUpdate %s", id)
	return f.get(ownerID, id)
}

func (f *fakeCharacters) LockOwner(ctx context.Context, ownerID string) error {
	f.add(ctx, "LockOwner %s", ownerID)
	return nil
}

func (f *fakeCharacters) CountActive(ctx context.Context, ownerID string) (int, error) {
	f.add(ctx, "CountActive %s", ownerID)
	return f.active, nil
}

func (f *fakeCharacters) Create(ctx context.Context, ownerID string, d domain.CharacterDraft, slugBase string) (domain.Persona, error) {
	f.add(ctx, "Create %s %q", slugBase, d.Name)
	f.nextID++
	p := domain.Persona{ID: fmt.Sprintf("new-%d", f.nextID), Slug: slugBase + "-0123456789", Name: d.Name, Era: d.Era,
		ShortBio: d.ShortBio, SystemPrompt: d.Prompt, OwnerID: ownerID}
	f.byID[p.ID] = p
	return p, nil
}

func (f *fakeCharacters) Update(ctx context.Context, p domain.Persona) (domain.Persona, error) {
	f.add(ctx, "Update %s archived=%v", p.ID, p.ArchivedAt != nil)
	f.byID[p.ID] = p
	return p, nil
}

func newCharacters() *fakeCharacters {
	removed := domain.Persona{ID: "removed", OwnerID: "ada", Name: "Removed", SystemPrompt: "x", RemovedAt: &archivedAt, RemovedReason: "spam"}
	return &fakeCharacters{byID: map[string]domain.Persona{
		"active":   {ID: "active", OwnerID: "ada", Name: "Ada", SystemPrompt: "You are Ada."},
		"used":     {ID: "used", OwnerID: "ada", Name: "Used", SystemPrompt: "x", HasConversations: true},
		"archived": {ID: "archived", OwnerID: "ada", Name: "Archived", SystemPrompt: "x", ArchivedAt: &archivedAt},
		"removed":  removed,
		"bo's":     {ID: "bo's", OwnerID: "bo", Name: "Bo's", SystemPrompt: "x"},
	}}
}

var draft = domain.CharacterDraft{Name: "  Émilie du Châtelet ", Era: "1740s", ShortBio: "Physicist.", Prompt: " You are Émilie. "}

func TestCreateCharacter(t *testing.T) {
	t.Run("validates, then locks, counts and creates in one transaction", func(t *testing.T) {
		chars := newCharacters()
		chars.active = 2
		txm := &portstest.TxManager{}
		p, err := NewCreateCharacter(txm, chars, 3).Execute(t.Context(), "ada", draft)
		if err != nil {
			t.Fatal(err)
		}
		want := []string{"LockOwner ada (tx)", "CountActive ada (tx)", `Create milie-du-ch-telet "Émilie du Châtelet" (tx)`}
		if !reflect.DeepEqual(chars.calls, want) || txm.Calls != 1 {
			t.Errorf("calls = %q, transactions %d; want %q", chars.calls, txm.Calls, want)
		}
		if p.Name != "Émilie du Châtelet" || p.SystemPrompt != "You are Émilie." || p.OwnerID != "ada" {
			t.Errorf("Execute() = %#v; want the trimmed draft", p)
		}
	})
	t.Run("at the cap nothing is written", func(t *testing.T) {
		chars := newCharacters()
		chars.active = 3
		_, err := NewCreateCharacter(&portstest.TxManager{}, chars, 3).Execute(t.Context(), "ada", draft)
		if !errors.Is(err, domain.ErrCharacterLimit) {
			t.Fatalf("err = %v, want ErrCharacterLimit", err)
		}
		if want := []string{"LockOwner ada (tx)", "CountActive ada (tx)"}; !reflect.DeepEqual(chars.calls, want) {
			t.Errorf("calls = %q, want %q", chars.calls, want)
		}
	})
	t.Run("an invalid draft is refused before any lookup", func(t *testing.T) {
		chars := newCharacters()
		txm := &portstest.TxManager{}
		_, err := NewCreateCharacter(txm, chars, 3).Execute(t.Context(), "ada", domain.CharacterDraft{Name: "Ada", Prompt: " "})
		if !errors.Is(err, domain.ErrCharacterPrompt) || len(chars.calls) > 0 || txm.Calls != 0 {
			t.Errorf("err = %v, calls %q, transactions %d", err, chars.calls, txm.Calls)
		}
	})
}

func TestUpdateCharacter(t *testing.T) {
	tests := []struct {
		name      string
		id        string
		patch     domain.CharacterPatch
		wantErr   error
		wantCalls []string
	}{
		{"edits inside the transaction after the lookup", "active", domain.CharacterPatch{Name: ptr("Augusta")}, nil,
			[]string{"GetOwn active", "GetOwnForUpdate active (tx)", "Update active archived=false (tx)"}},
		{"a used character's name is locked", "used", domain.CharacterPatch{Name: ptr("Other")}, domain.ErrNameLocked,
			[]string{"GetOwn used", "GetOwnForUpdate used (tx)"}},
		{"a used character's bio can change", "used", domain.CharacterPatch{ShortBio: ptr("New.")}, nil,
			[]string{"GetOwn used", "GetOwnForUpdate used (tx)", "Update used archived=false (tx)"}},
		{"an archived character stays archived", "archived", domain.CharacterPatch{ShortBio: ptr("New.")}, nil,
			[]string{"GetOwn archived", "GetOwnForUpdate archived (tx)", "Update archived archived=true (tx)"}},
		{"a removed character", "removed", domain.CharacterPatch{ShortBio: ptr("New.")}, domain.ErrCharacterRemoved,
			[]string{"GetOwn removed", "GetOwnForUpdate removed (tx)"}},
		{"invalid", "active", domain.CharacterPatch{Prompt: ptr("")}, domain.ErrCharacterPrompt,
			[]string{"GetOwn active", "GetOwnForUpdate active (tx)"}},
		{"someone else's is 404 before any lock", "bo's", domain.CharacterPatch{}, kernel.ErrNotFound, []string{"GetOwn bo's"}},
		{"missing", "missing", domain.CharacterPatch{}, kernel.ErrNotFound, []string{"GetOwn missing"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			chars := newCharacters()
			before := chars.byID[tt.id]
			txm := &portstest.TxManager{}
			_, err := NewUpdateCharacter(txm, chars).Execute(t.Context(), "ada", tt.id, tt.patch)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v, want %v", err, tt.wantErr)
			}
			if !reflect.DeepEqual(chars.calls, tt.wantCalls) {
				t.Errorf("calls = %q, want %q", chars.calls, tt.wantCalls)
			}
			if err != nil && !reflect.DeepEqual(chars.byID[tt.id], before) {
				t.Errorf("character changed after a refusal: %#v", chars.byID[tt.id])
			}
		})
	}
}

func TestArchiveCharacter(t *testing.T) {
	tests := []struct {
		name      string
		id        string
		wantErr   error
		wantCalls []string
	}{
		{"archives an active character", "active", nil,
			[]string{"GetOwn active", "GetOwnForUpdate active (tx)", "Update active archived=true (tx)"}},
		{"archiving again writes nothing", "archived", nil, []string{"GetOwn archived", "GetOwnForUpdate archived (tx)"}},
		{"a removed character", "removed", domain.ErrCharacterRemoved, []string{"GetOwn removed", "GetOwnForUpdate removed (tx)"}},
		{"someone else's", "bo's", kernel.ErrNotFound, []string{"GetOwn bo's"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			chars := newCharacters()
			p, err := NewArchiveCharacter(&portstest.TxManager{}, chars).Execute(t.Context(), "ada", tt.id)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v, want %v", err, tt.wantErr)
			}
			if !reflect.DeepEqual(chars.calls, tt.wantCalls) {
				t.Errorf("calls = %q, want %q", chars.calls, tt.wantCalls)
			}
			if err == nil && p.Status() != domain.PersonaArchived {
				t.Errorf("Execute() = %#v, want archived", p)
			}
		})
	}
	t.Run("archiving again keeps the first time", func(t *testing.T) {
		p, err := NewArchiveCharacter(&portstest.TxManager{}, newCharacters()).Execute(t.Context(), "ada", "archived")
		if err != nil || !p.ArchivedAt.Equal(archivedAt) {
			t.Errorf("Execute() = %#v, %v", p, err)
		}
	})
}

func TestUnarchiveCharacter(t *testing.T) {
	tests := []struct {
		name      string
		id        string
		active    int
		wantErr   error
		wantCalls []string
	}{
		{"unarchives below the cap", "archived", 2, nil, []string{"GetOwn archived", "LockOwner ada (tx)",
			"GetOwnForUpdate archived (tx)", "CountActive ada (tx)", "Update archived archived=false (tx)"}},
		{"refused at the cap", "archived", 3, domain.ErrCharacterLimit, []string{"GetOwn archived", "LockOwner ada (tx)",
			"GetOwnForUpdate archived (tx)", "CountActive ada (tx)"}},
		{"an active character is a no-op, even at the cap", "active", 3, nil,
			[]string{"GetOwn active", "LockOwner ada (tx)", "GetOwnForUpdate active (tx)"}},
		{"a removed character", "removed", 0, domain.ErrCharacterRemoved,
			[]string{"GetOwn removed", "LockOwner ada (tx)", "GetOwnForUpdate removed (tx)"}},
		{"someone else's", "bo's", 0, kernel.ErrNotFound, []string{"GetOwn bo's"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			chars := newCharacters()
			chars.active = tt.active
			p, err := NewUnarchiveCharacter(&portstest.TxManager{}, chars, 3).Execute(t.Context(), "ada", tt.id)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v, want %v", err, tt.wantErr)
			}
			if !reflect.DeepEqual(chars.calls, tt.wantCalls) {
				t.Errorf("calls = %q, want %q", chars.calls, tt.wantCalls)
			}
			if err == nil && p.Status() != domain.PersonaActive {
				t.Errorf("Execute() = %#v, want active", p)
			}
		})
	}
}

func TestListCharacters(t *testing.T) {
	chars := newCharacters()
	l, err := NewListCharacters(chars, 20).Execute(t.Context(), "ada")
	// active and used are active; archived and removed aren't; bo's isn't ada's.
	if err != nil || len(l.Characters) != 4 || l.Active != 2 || l.Limit != 20 {
		t.Errorf("Execute() = %+v, %v", l, err)
	}
}

func ptr[T any](v T) *T { return &v }

// unusable is a character that can't take part in a discussion.
func unusable(archived bool) domain.Persona {
	p := domain.Persona{ID: "p9", Name: "Ada", SystemPrompt: "You are Ada.", OwnerID: "u1"}
	if archived {
		p.ArchivedAt = &archivedAt
	} else {
		p.RemovedAt, p.RemovedReason = &archivedAt, "spam"
	}
	return p
}

func TestStartConversationRefusesUnusableCharacters(t *testing.T) {
	for _, tt := range []struct {
		name     string
		archived bool
		want     error
	}{
		{"archived", true, domain.ErrCharacterArchived},
		{"removed", false, domain.ErrCharacterRemoved},
	} {
		t.Run(tt.name, func(t *testing.T) {
			convs, model, out := &fakeConversations{}, &fakeModel{chunks: []string{"Hi."}}, &fakeStream{}
			txm := &portstest.TxManager{}
			quotes := &fakeQuotes{quotes: map[string]quote.Quote{reacted.ID: reacted}}
			persona := unusable(tt.archived)
			_, err := NewStartConversation(txm, quotes, &fakePersonas{persona: persona}, convs, model).
				Execute(t.Context(), "u1", reacted.ID, persona.ID, out)
			if !errors.Is(err, tt.want) {
				t.Fatalf("err = %v, want %v", err, tt.want)
			}
			if len(convs.created) > 0 || txm.Calls != 0 || model.calls > 0 || out.started != "" {
				t.Errorf("created %v, transactions %d, model calls %d, started %q; want nothing",
					convs.created, txm.Calls, model.calls, out.started)
			}
		})
	}
}

func TestPostMessageRefusals(t *testing.T) {
	answered := []domain.Message{{ID: 1, Role: domain.RoleAssistant, Content: "Do you, though?"}}
	removedConv := domain.Conversation{ID: "c1", QuoteID: reacted.ID, Persona: diogenes, Messages: answered,
		RemovedAt: &archivedAt, RemovedReason: "Harassment"}
	// Both removed: the conversation is checked first.
	removedBoth := removedConv
	removedBoth.Persona = unusable(false)
	// Not the reader's turn either: the removal is reported, not the turn.
	removedWaiting := removedConv
	removedWaiting.Messages = append(append([]domain.Message{}, answered...), domain.Message{ID: 2, Role: domain.RoleUser})
	tests := []struct {
		name string
		conv domain.Conversation
		want error
	}{
		{"(D13) a removed discussion", removedConv, domain.ErrConversationRemoved},
		{"(D13) the discussion's removal comes before its character's", removedBoth, domain.ErrConversationRemoved},
		{"(D13) the removal comes before the turn", removedWaiting, domain.ErrConversationRemoved},
		{"an archived character", domain.Conversation{ID: "c1", QuoteID: reacted.ID, Persona: unusable(true), Messages: answered},
			domain.ErrCharacterArchived},
		{"a removed character", domain.Conversation{ID: "c1", QuoteID: reacted.ID, Persona: unusable(false), Messages: answered},
			domain.ErrCharacterRemoved},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			model := &fakeModel{chunks: []string{"Hm."}}
			uc, convs, out := newPost(tt.conv, model)
			if _, err := uc.Execute(t.Context(), "u1", "c1", "Sometimes.", out); !errors.Is(err, tt.want) {
				t.Fatalf("err = %v, want %v", err, tt.want)
			}
			if len(convs.appended) > 0 || model.calls > 0 || out.started != "" {
				t.Errorf("appended %v, model calls %d, started %q; want nothing", convs.appended, model.calls, out.started)
			}
		})
	}
}

// R-RACE: when the character or the discussion became unusable while the
// reply streamed, the reply is discarded with that reason, not a generic
// failure, and what was stored for it is undone.
func TestReplyRefusedWhileStreaming(t *testing.T) {
	answered := domain.Conversation{ID: "c1", QuoteID: reacted.ID, Persona: diogenes,
		Messages: []domain.Message{{ID: 1, Role: domain.RoleAssistant, Content: "Do you, though?"}}}
	for _, refusal := range []error{domain.ErrCharacterArchived, domain.ErrCharacterRemoved, domain.ErrConversationRemoved} {
		t.Run("start: "+refusal.Error(), func(t *testing.T) {
			f := newStart(&fakeModel{chunks: []string{"Hm."}})
			f.convs.appendAssistantErr = refusal
			_, err := f.uc.Execute(t.Context(), "u1", reacted.ID, diogenes.ID, f.out)
			if !errors.Is(err, refusal) || errors.Is(err, domain.ErrReplyNotSaved) {
				t.Fatalf("err = %v, want only %v", err, refusal)
			}
			if !reflect.DeepEqual(f.convs.deleted, []string{"conv-q1"}) || len(f.convs.outsideTx) > 0 {
				t.Errorf("deleted %v, writes outside a transaction %v", f.convs.deleted, f.convs.outsideTx)
			}
		})
		t.Run("post: "+refusal.Error(), func(t *testing.T) {
			uc, convs, out := newPost(answered, &fakeModel{chunks: []string{"Hm."}})
			convs.appendAssistantErr = refusal
			_, err := uc.Execute(t.Context(), "u1", "c1", "Sometimes.", out)
			if !errors.Is(err, refusal) || errors.Is(err, domain.ErrReplyNotSaved) {
				t.Fatalf("err = %v, want only %v", err, refusal)
			}
			if len(convs.appended) != 1 || !reflect.DeepEqual(convs.deletedMessages, []int64{convs.appended[0].ID}) {
				t.Errorf("appended %v, deleted messages %v; want the reader's message undone", convs.appended, convs.deletedMessages)
			}
		})
	}
}
