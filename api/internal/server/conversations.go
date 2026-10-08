package server

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/an4eetos/serendipity/api/internal/auth"
	"github.com/an4eetos/serendipity/api/internal/llm"
	"github.com/an4eetos/serendipity/api/internal/store"
)

const maxUserMessage = 4000

// Conversation endpoints stream the character's reply as server-sent events:
//
//	data: {"type":"start","conversation_id":"…"}
//	data: {"type":"delta","text":"…"}            (repeated)
//	data: {"type":"done","message":{…}}
//	data: {"type":"error","error":"…"}          (instead of done)
type event struct {
	Type           string         `json:"type"`
	ConversationID string         `json:"conversation_id,omitempty"`
	Text           string         `json:"text,omitempty"`
	Message        *store.Message `json:"message,omitempty"`
	Error          string         `json:"error,omitempty"`
}

type createConversationRequest struct {
	QuoteID   string `json:"quote_id"`
	PersonaID string `json:"persona_id"`
	Mode      string `json:"mode"`
}

// createConversation starts a conversation about a quote and streams the
// character's first reaction.
func (s *Server) createConversation(w http.ResponseWriter, r *http.Request) {
	var req createConversationRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, r, err)
		return
	}
	if req.Mode == "" {
		req.Mode = llm.ModeReact
	}
	if !llm.ValidMode(req.Mode) {
		writeError(w, r, badRequest("mode must be react or argue"))
		return
	}
	ctx, uid := r.Context(), auth.UserID(r.Context())
	q, err := s.store.GetQuote(ctx, uid, req.QuoteID)
	if err != nil {
		writeError(w, r, err)
		return
	}
	persona, err := s.store.GetPersona(ctx, uid, req.PersonaID)
	if err != nil {
		writeError(w, r, err)
		return
	}
	convID, err := s.store.CreateConversation(ctx, uid, q.ID, persona.ID, req.Mode)
	if err != nil {
		writeError(w, r, err)
		return
	}

	stream, err := newSSE(w)
	if err != nil {
		writeError(w, r, err)
		return
	}
	_ = stream.send(event{Type: "start", ConversationID: convID})

	ok := s.streamReply(ctx, stream, convID, llm.Request{
		PersonaName:   persona.Name,
		PersonaPrompt: persona.SystemPrompt,
		Mode:          req.Mode,
		Passage:       passageOf(q),
	})
	if !ok {
		// Don't leave an empty conversation behind.
		if err := s.store.DeleteConversation(context.WithoutCancel(ctx), uid, convID); err != nil {
			slog.Error("delete failed conversation", "err", err)
		}
	}
}

type postMessageRequest struct {
	Content string `json:"content"`
}

// postMessage adds the reader's reply and streams the character's answer.
func (s *Server) postMessage(w http.ResponseWriter, r *http.Request) {
	var req postMessageRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, r, err)
		return
	}
	req.Content = strings.TrimSpace(req.Content)
	if req.Content == "" || len([]rune(req.Content)) > maxUserMessage {
		writeError(w, r, badRequest("message must be 1–%d characters", maxUserMessage))
		return
	}
	ctx, uid := r.Context(), auth.UserID(r.Context())
	conv, err := s.store.GetConversation(ctx, uid, chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, r, err)
		return
	}
	if n := len(conv.Messages); n == 0 || conv.Messages[n-1].Role != "assistant" {
		writeError(w, r, apiError{http.StatusConflict, "wait for the character to answer first"})
		return
	}
	q, err := s.store.GetQuote(ctx, uid, conv.QuoteID)
	if err != nil {
		writeError(w, r, err)
		return
	}
	userMsg, err := s.store.AppendMessage(ctx, conv.ID, "user", req.Content)
	if err != nil {
		writeError(w, r, err)
		return
	}

	history := make([]llm.Turn, 0, len(conv.Messages)+1)
	for _, m := range conv.Messages {
		history = append(history, llm.Turn{Role: m.Role, Content: m.Content})
	}
	history = append(history, llm.Turn{Role: "user", Content: userMsg.Content})

	stream, err := newSSE(w)
	if err != nil {
		writeError(w, r, err)
		return
	}
	_ = stream.send(event{Type: "start", ConversationID: conv.ID})
	ok := s.streamReply(ctx, stream, conv.ID, llm.Request{
		PersonaName:   conv.Persona.Name,
		PersonaPrompt: conv.Persona.SystemPrompt,
		Mode:          conv.Mode,
		Passage:       passageOf(q),
		History:       history,
	})
	if !ok {
		// Drop the unanswered message so the history keeps alternating and
		// the reader can simply send it again.
		if err := s.store.DeleteMessage(context.WithoutCancel(ctx), conv.ID, userMsg.ID); err != nil {
			slog.Error("delete unanswered message", "err", err)
		}
	}
}

// streamReply runs the model, forwards text chunks, stores the final reply
// and reports whether it succeeded.
func (s *Server) streamReply(ctx context.Context, stream *sse, convID string, req llm.Request) bool {
	reply, err := s.llm.Stream(ctx, req, func(text string) error {
		return stream.send(event{Type: "delta", Text: text})
	})
	if err != nil {
		msg := "the character could not answer right now — try again"
		if errors.Is(err, llm.ErrRefused) {
			msg = llm.ErrRefused.Error()
		}
		if ctx.Err() == nil {
			slog.Error("character reply failed", "conversation", convID, "err", err)
		}
		_ = stream.send(event{Type: "error", Error: msg})
		return false
	}
	m, err := s.store.AppendMessage(context.WithoutCancel(ctx), convID, "assistant", reply)
	if err != nil {
		slog.Error("store reply", "conversation", convID, "err", err)
		_ = stream.send(event{Type: "error", Error: "could not save the reply"})
		return false
	}
	_ = stream.send(event{Type: "done", Message: &m})
	return true
}

func passageOf(q store.Quote) llm.Passage {
	p := llm.Passage{Text: q.Text, Location: q.Location, Note: q.Note}
	if q.Work != nil {
		p.Title = q.Work.Title
		p.Authors = q.Work.Authors
	}
	return p
}
