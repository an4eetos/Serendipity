package anthropic

import (
	"testing"

	"github.com/anthropics/anthropic-sdk-go"

	"github.com/an4eetos/serendipity/backend/internal/discussion/domain"
)

func TestBuildMessagesAlternates(t *testing.T) {
	msgs := buildMessages([]domain.Turn{
		{Role: domain.RoleUser, Content: "opening"},
		{Role: domain.RoleAssistant, Content: "reply"},
		{Role: domain.RoleUser, Content: "answer"},
	})
	wantRoles := []anthropic.BetaMessageParamRole{
		anthropic.BetaMessageParamRoleUser,
		anthropic.BetaMessageParamRoleAssistant,
		anthropic.BetaMessageParamRoleUser,
	}
	if len(msgs) != len(wantRoles) {
		t.Fatalf("got %d messages, want %d", len(msgs), len(wantRoles))
	}
	for i, m := range msgs {
		if m.Role != wantRoles[i] {
			t.Errorf("message %d role = %s, want %s", i, m.Role, wantRoles[i])
		}
	}
}
