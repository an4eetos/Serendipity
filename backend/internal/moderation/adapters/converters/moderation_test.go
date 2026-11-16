package converters

import (
	"encoding/json"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	discussion "github.com/an4eetos/serendipity/backend/internal/discussion/domain"
	"github.com/an4eetos/serendipity/backend/internal/moderation/domain"
	quote "github.com/an4eetos/serendipity/backend/internal/quote/domain"
)

// promptFields returns the paths of the fields of typ, followed through
// embedded structs, pointers and slices, whose JSON name is prompt or
// system_prompt.
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
			if slices.Contains([]string{"prompt", "system_prompt"}, name) {
				out = append(out, path+"."+f.Name)
			}
			walk(f.Type, path+"."+f.Name, seen)
		}
	}
	walk(typ, typ.Name(), map[reflect.Type]bool{})
	return out
}

// R5: the reporter's response carries no prompt; admin responses carry it
// only through the embedded character.
func TestPromptOnlyInAdminResponses(t *testing.T) {
	for _, v := range []any{ReportResponse{}, TakedownResponse{}, DismissResponse{}, ReportEntryResponse{}} {
		if got := promptFields(reflect.TypeOf(v)); len(got) > 0 {
			t.Errorf("%T has prompt fields %v", v, got)
		}
	}
	for _, tt := range []struct {
		typ  reflect.Type
		want string
	}{
		{reflect.TypeOf(ReportedTargetResponse{}), "ReportedTargetResponse.Character.Prompt"},
		{reflect.TypeOf(TakedownTargetResponse{}), "TakedownTargetResponse.Character.Prompt"},
	} {
		if got := promptFields(tt.typ); !reflect.DeepEqual(got, []string{tt.want}) {
			t.Errorf("%s prompt fields = %v, want only %s", tt.typ, got, tt.want)
		}
	}
}

const secret = "SECRET-PROMPT-91c2"

var at = time.Date(2026, 11, 16, 0, 0, 0, 0, time.UTC)

func TestCharacterContentForAdmins(t *testing.T) {
	p := discussion.Persona{ID: "p1", Slug: "ada-0123456789", Name: "Ada", Era: "1840s", ShortBio: "Countess.",
		SystemPrompt: secret, OwnerID: "owner", RemovedAt: &at, RemovedReason: "living person"}
	target := domain.CharacterTarget(discussion.ModeratedPersona{Persona: p})
	rts := ToReportedTargetResponses([]domain.ReportedTarget{{Ref: target.Ref, OwnerID: "owner", Content: &target}})
	got := rts[0]
	if got.TargetKind != "character" || got.Quote != nil || got.Conversation != nil || got.Character == nil {
		t.Fatalf("ReportedTargetResponse = %+v; want only the character", got)
	}
	want := AdminCharacterResponse{ID: "p1", Slug: "ada-0123456789", Name: "Ada", Era: "1840s", ShortBio: "Countess.",
		Prompt: secret, Status: "removed", RemovedAt: &at, RemovedReason: "living person"}
	if !reflect.DeepEqual(*got.Character, want) {
		t.Errorf("character = %+v, want %+v", *got.Character, want)
	}

	tds := ToTakedownTargetResponses([]domain.TakedownView{{Takedown: domain.Takedown{ID: "td1", Target: target.Ref}, Content: &target}})
	if tds[0].Character == nil || tds[0].Character.Prompt != secret || tds[0].Quote != nil || tds[0].Conversation != nil {
		t.Errorf("TakedownTargetResponse = %+v", tds[0])
	}
}

// A discussion's content embeds the conversation and its quote, never a
// character or its prompt.
func TestConversationContentHasNoCharacter(t *testing.T) {
	persona := discussion.Persona{ID: "p1", Name: "Ada", SystemPrompt: secret, OwnerID: "owner"}
	conv := discussion.Conversation{ID: "c1", QuoteID: "q1", UserID: "owner", Persona: persona,
		Messages: []discussion.Message{{Role: discussion.RoleAssistant, Content: "Hm."}}}
	target := domain.ConversationTarget(conv, quote.Quote{ID: "q1", UserID: "owner", Visibility: quote.VisibilityPublic})
	rts := ToReportedTargetResponses([]domain.ReportedTarget{{Ref: target.Ref, Content: &target}})
	if rts[0].Character != nil || rts[0].Conversation == nil || rts[0].Quote == nil {
		t.Errorf("ReportedTargetResponse = %+v; want quote and discussion, no character", rts[0])
	}
	deleted := ToReportedTargetResponses([]domain.ReportedTarget{{Ref: target.Ref}})
	if deleted[0].Character != nil || deleted[0].Conversation != nil || deleted[0].Quote != nil {
		t.Errorf("deleted content = %+v; want all null", deleted[0])
	}
	raw, err := json.Marshal(rts)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), secret) {
		t.Errorf("a discussion's admin JSON carries its character's prompt: %s", raw)
	}
	report := ToReportResponse(domain.Report{ID: "r1", Target: domain.TargetRef{Kind: domain.TargetCharacter, ID: "p1"},
		Reason: domain.ReasonLivingPerson})
	if raw, _ := json.Marshal(report); strings.Contains(string(raw), secret) || report.Reason != "living_person" {
		t.Errorf("ReportResponse = %s", raw)
	}
}
