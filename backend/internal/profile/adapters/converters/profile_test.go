package converters

import (
	"encoding/json"
	"strings"
	"testing"

	discussion "github.com/an4eetos/serendipity/backend/internal/discussion/domain"
	"github.com/an4eetos/serendipity/backend/internal/profile/domain"
	quote "github.com/an4eetos/serendipity/backend/internal/quote/domain"
)

// R5: a public profile shows a custom character's name, era and bio next to
// its discussions, never its prompt.
func TestPublicProfileCarriesNoPrompt(t *testing.T) {
	const secret = "SECRET-PROMPT-c55e"
	persona := discussion.Persona{ID: "p1", Name: "Ada", Era: "1840s", ShortBio: "Countess.", SystemPrompt: secret, OwnerID: "u1"}
	conv := discussion.Conversation{ID: "c1", QuoteID: "q1", Persona: persona,
		Messages: []discussion.Message{{Role: discussion.RoleAssistant, Content: "Hm."}}}
	resp := ToPublicProfileResponse(domain.PublicProfile{
		Quotes: []discussion.Thread{{Quote: quote.Quote{ID: "q1"}, Conversations: []discussion.Conversation{conv}}},
	})
	raw, err := json.Marshal(resp)
	if err != nil {
		t.Fatal(err)
	}
	body := string(raw)
	if strings.Contains(body, secret) || strings.Contains(body, `"prompt"`) || strings.Contains(body, `"system_prompt"`) {
		t.Errorf("public profile JSON carries a prompt: %s", body)
	}
	if !strings.Contains(body, `"name":"Ada","era":"1840s","short_bio":"Countess."`) {
		t.Errorf("public profile JSON lacks the character's name, era and bio: %s", body)
	}
}
