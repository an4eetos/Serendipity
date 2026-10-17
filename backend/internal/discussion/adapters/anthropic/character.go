// Package anthropic implements ports.CharacterModel on Claude.
package anthropic

import (
	"context"
	"errors"
	"strings"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"

	"github.com/an4eetos/serendipity/backend/internal/discussion/domain"
)

// Character streams character replies from Claude.
type Character struct {
	api   anthropic.Client
	model string
}

func NewCharacter(apiKey, model string) *Character {
	var opts []option.RequestOption
	if apiKey != "" {
		opts = append(opts, option.WithAPIKey(apiKey))
	}
	return &Character{api: anthropic.NewClient(opts...), model: model}
}

func buildMessages(turns []domain.Turn) []anthropic.BetaMessageParam {
	msgs := make([]anthropic.BetaMessageParam, 0, len(turns))
	for _, t := range turns {
		block := anthropic.NewBetaTextBlock(t.Content)
		if t.Role == domain.RoleAssistant {
			msgs = append(msgs, anthropic.BetaMessageParam{
				Role:    anthropic.BetaMessageParamRoleAssistant,
				Content: []anthropic.BetaContentBlockParamUnion{block},
			})
		} else {
			msgs = append(msgs, anthropic.NewBetaUserMessage(block))
		}
	}
	return msgs
}

// Reply opts into server-side refusal fallbacks ("default" routing): if the
// primary model's safeguards decline, a fallback model continues on the same
// stream. domain.ErrRefused is returned only when nothing served the request.
func (c *Character) Reply(ctx context.Context, p domain.CharacterPrompt, onText func(string) error) (string, error) {
	params := anthropic.BetaMessageNewParams{
		Model:     anthropic.Model(c.model),
		MaxTokens: 4096,
		System:    []anthropic.BetaTextBlockParam{{Text: p.System}},
		Messages:  buildMessages(p.Turns),
		// Short conversational replies: low effort keeps them quick.
		OutputConfig: anthropic.BetaOutputConfigParam{Effort: anthropic.BetaOutputConfigEffortLow},
		Betas:        []anthropic.AnthropicBeta{"server-side-fallback-2026-07-01"},
	}
	stream := c.api.Beta.Messages.NewStreaming(ctx, params, option.WithJSONSet("fallbacks", "default"))
	defer stream.Close()

	var msg anthropic.BetaMessage
	for stream.Next() {
		event := stream.Current()
		if err := msg.Accumulate(event); err != nil {
			return "", err
		}
		if delta, ok := event.AsAny().(anthropic.BetaRawContentBlockDeltaEvent); ok {
			if text, ok := delta.Delta.AsAny().(anthropic.BetaTextDelta); ok && text.Text != "" {
				if err := onText(text.Text); err != nil {
					return "", err
				}
			}
		}
	}
	if err := stream.Err(); err != nil {
		return "", err
	}
	if msg.StopReason == anthropic.BetaStopReasonRefusal {
		return "", domain.ErrRefused
	}

	var b strings.Builder
	for _, block := range msg.Content {
		if text, ok := block.AsAny().(anthropic.BetaTextBlock); ok {
			b.WriteString(text.Text)
		}
	}
	reply := strings.TrimSpace(b.String())
	if reply == "" {
		return "", errors.New("empty reply")
	}
	return reply, nil
}
