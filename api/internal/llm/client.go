package llm

import (
	"context"
	"errors"
	"strings"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
)

// ErrRefused means the model declined to answer and no fallback served it.
var ErrRefused = errors.New("the character declined to respond to this")

// Client streams character replies from Claude.
type Client struct {
	api   anthropic.Client
	model string
}

// New creates a client. The API key comes from ANTHROPIC_API_KEY.
func New(model string) *Client {
	return &Client{api: anthropic.NewClient(), model: model}
}

// Request is everything needed to produce the next character reply.
type Request struct {
	PersonaName   string
	PersonaPrompt string
	Mode          string
	Passage       Passage
	// History holds the stored turns, starting with the character's first
	// reaction and ending with the reader's latest message (if any).
	History []Turn
}

func buildMessages(req Request) []anthropic.BetaMessageParam {
	msgs := []anthropic.BetaMessageParam{
		anthropic.NewBetaUserMessage(anthropic.NewBetaTextBlock(OpeningMessage(req.Passage))),
	}
	for _, t := range req.History {
		block := anthropic.NewBetaTextBlock(t.Content)
		if t.Role == "assistant" {
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

// Stream generates the character's next reply, calling onText with each
// chunk as it arrives, and returns the full reply.
//
// Requests opt into server-side refusal fallbacks ("default" routing): if the
// primary model's safeguards decline, a fallback model continues on the same
// stream. ErrRefused is returned only when nothing served the request; any
// partial text already sent must then be discarded by the caller.
func (c *Client) Stream(ctx context.Context, req Request, onText func(string) error) (string, error) {
	params := anthropic.BetaMessageNewParams{
		Model:     anthropic.Model(c.model),
		MaxTokens: 4096,
		System:    []anthropic.BetaTextBlockParam{{Text: BuildSystem(req.PersonaName, req.PersonaPrompt, req.Mode)}},
		Messages:  buildMessages(req),
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
		return "", ErrRefused
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
