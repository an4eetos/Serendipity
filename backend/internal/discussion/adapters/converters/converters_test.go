package converters

import (
	"encoding/json"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/an4eetos/serendipity/backend/internal/discussion/domain"
	quote "github.com/an4eetos/serendipity/backend/internal/quote/domain"
)

// promptTags are the JSON names a character's prompt could leak under.
var promptTags = []string{"prompt", "system_prompt"}

// promptFields returns the paths of the fields of typ, followed through
// embedded structs, pointers, slices and maps, whose JSON name is a prompt
// tag.
func promptFields(typ reflect.Type) []string {
	var out []string
	var walk func(t reflect.Type, path string, seen map[reflect.Type]bool)
	walk = func(t reflect.Type, path string, seen map[reflect.Type]bool) {
		for t.Kind() == reflect.Pointer || t.Kind() == reflect.Slice || t.Kind() == reflect.Array || t.Kind() == reflect.Map {
			t = t.Elem()
		}
		if t.Kind() != reflect.Struct || seen[t] {
			return
		}
		seen[t] = true
		defer delete(seen, t)
		for i := range t.NumField() {
			f := t.Field(i)
			name := strings.Split(f.Tag.Get("json"), ",")[0]
			if slices.Contains(promptTags, name) {
				out = append(out, path+"."+f.Name)
			}
			walk(f.Type, path+"."+f.Name, seen)
		}
	}
	walk(typ, typ.Name(), map[reflect.Type]bool{})
	return out
}

// R5: no response another reader can get carries a character's prompt.
func TestReaderResponsesCarryNoPrompt(t *testing.T) {
	for _, v := range []any{PersonaResponse{}, ConversationResponse{}, ThreadResponse{}, MessageResponse{}, Event{}} {
		if got := promptFields(reflect.TypeOf(v)); len(got) > 0 {
			t.Errorf("%T has prompt fields %v", v, got)
		}
	}
	// The owner's own view is the one response with the prompt; this keeps
	// the check above honest.
	if got := promptFields(reflect.TypeOf(CharacterListResponse{})); !reflect.DeepEqual(got, []string{"CharacterListResponse.Characters.Prompt"}) {
		t.Errorf("CharacterListResponse prompt fields = %v", got)
	}
}

const secret = "SECRET-PROMPT-7f3a"

func customCharacter() domain.Persona {
	at := time.Date(2026, 11, 16, 0, 0, 0, 0, time.UTC)
	return domain.Persona{ID: "p1", Slug: "ada-0123456789", Name: "Ada", Era: "1840s", ShortBio: "Countess.",
		SystemPrompt: secret, OwnerID: "owner", ArchivedAt: &at, HasConversations: true, CreatedAt: at, UpdatedAt: at}
}

func TestReaderJSONCarriesNoPrompt(t *testing.T) {
	p := customCharacter()
	conv := domain.Conversation{ID: "c1", QuoteID: "q1", Persona: p, Messages: []domain.Message{{ID: 1, Role: domain.RoleAssistant, Content: "Hm."}}}
	for name, v := range map[string]any{
		"persona":       ToPersonaResponse(p),
		"personas":      ToPersonaResponses([]domain.Persona{p}),
		"conversations": ToConversationResponses([]domain.Conversation{conv}),
		"thread":        ToThreadResponse(domain.Thread{Quote: quote.Quote{ID: "q1"}, Conversations: []domain.Conversation{conv}}),
	} {
		raw, err := json.Marshal(v)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(raw), secret) {
			t.Errorf("%s JSON carries the prompt: %s", name, raw)
		}
	}
	if got := ToPersonaResponse(p); got.Status != "archived" || got.IsPreset {
		t.Errorf("ToPersonaResponse() = %+v, want status archived", got)
	}
}

func TestToCharacterResponse(t *testing.T) {
	p := customCharacter()
	got := ToCharacterResponse(p)
	if got.Prompt != secret || got.Status != "archived" || !got.NameLocked || got.ArchivedAt == nil ||
		got.ID != "p1" || got.Slug != p.Slug || got.Era != "1840s" || got.ShortBio != "Countess." {
		t.Errorf("ToCharacterResponse() = %+v", got)
	}

	empty := ToCharacterListResponse(domain.NewCharacterList(nil, 20))
	raw, err := json.Marshal(empty)
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != `{"characters":[],"active":0,"active_limit":20}` {
		t.Errorf("empty list JSON = %s; characters must never be null", raw)
	}
	list := ToCharacterListResponse(domain.NewCharacterList([]domain.Persona{p, {ID: "p2", Name: "Bo", SystemPrompt: "x"}}, 3))
	if len(list.Characters) != 2 || list.Active != 1 || list.ActiveLimit != 3 {
		t.Errorf("ToCharacterListResponse() = %+v", list)
	}
}

func TestCharacterRequests(t *testing.T) {
	var req CharacterPatchRequest
	if err := json.Unmarshal([]byte(`{"short_bio":"","prompt":"New"}`), &req); err != nil {
		t.Fatal(err)
	}
	patch := req.ToPatch()
	// Omitted fields stay nil (unchanged); an empty string clears the field.
	if patch.Name != nil || patch.Era != nil || patch.ShortBio == nil || *patch.ShortBio != "" || patch.Prompt == nil || *patch.Prompt != "New" {
		t.Errorf("ToPatch() = %+v", patch)
	}
	d := CharacterRequest{Name: "Ada", Era: "1840s", ShortBio: "Countess.", Prompt: "You are Ada."}.ToDraft()
	if d != (domain.CharacterDraft{Name: "Ada", Era: "1840s", ShortBio: "Countess.", Prompt: "You are Ada."}) {
		t.Errorf("ToDraft() = %+v", d)
	}
}
