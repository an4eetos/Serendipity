// Package handlers serves characters, discussions and quote pages over
// HTTP.
package handlers

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/an4eetos/serendipity/backend/internal/discussion/adapters/converters"
	"github.com/an4eetos/serendipity/backend/internal/discussion/domain"
	"github.com/an4eetos/serendipity/backend/internal/discussion/usecase"
	"github.com/an4eetos/serendipity/backend/internal/kernel/adapters/httpapi"
)

type Handler struct {
	personas *usecase.ListPersonas
	thread   *usecase.GetThread
	start    *usecase.StartConversation
	post     *usecase.PostMessage
}

func New(personas *usecase.ListPersonas, thread *usecase.GetThread, start *usecase.StartConversation, post *usecase.PostMessage) *Handler {
	return &Handler{personas: personas, thread: thread, start: start, post: post}
}

func (h *Handler) RegisterRoutes(_, private chi.Router) {
	private.Get("/quotes/{id}", h.getQuote)
	private.Get("/personas", h.listPersonas)
	private.Post("/conversations", h.createConversation)
	private.Post("/conversations/{id}/messages", h.postMessage)
}

func (h *Handler) getQuote(w http.ResponseWriter, r *http.Request) {
	t, err := h.thread.Execute(r.Context(), httpapi.UserID(r.Context()), chi.URLParam(r, "id"))
	if err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, converters.ToThreadResponse(t))
}

func (h *Handler) listPersonas(w http.ResponseWriter, r *http.Request) {
	ps, err := h.personas.Execute(r.Context(), httpapi.UserID(r.Context()))
	if err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, converters.ToPersonaResponses(ps))
}

func (h *Handler) createConversation(w http.ResponseWriter, r *http.Request) {
	var req converters.StartRequest
	if err := httpapi.DecodeJSON(r, &req); err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	out := &sseReply{w: w}
	m, err := h.start.Execute(r.Context(), httpapi.UserID(r.Context()), req.QuoteID, req.PersonaID, out)
	out.finish(r, m, err)
}

func (h *Handler) postMessage(w http.ResponseWriter, r *http.Request) {
	var req converters.PostMessageRequest
	if err := httpapi.DecodeJSON(r, &req); err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	out := &sseReply{w: w}
	m, err := h.post.Execute(r.Context(), httpapi.UserID(r.Context()), chi.URLParam(r, "id"), req.Content, out)
	out.finish(r, m, err)
}

// sseReply implements ports.ReplyStream as server-sent events. Until the
// stream has started, failures are answered with a plain error response.
type sseReply struct {
	w              http.ResponseWriter
	stream         *httpapi.Stream
	conversationID string
}

func (s *sseReply) Started(conversationID string) error {
	stream, err := httpapi.NewStream(s.w)
	if err != nil {
		return err
	}
	s.stream, s.conversationID = stream, conversationID
	_ = stream.Send(converters.StartEvent(conversationID))
	return nil
}

func (s *sseReply) Text(chunk string) error {
	return s.stream.Send(converters.DeltaEvent(chunk))
}

func (s *sseReply) finish(r *http.Request, m domain.Message, err error) {
	if s.stream == nil {
		httpapi.WriteError(s.w, r, err)
		return
	}
	if err != nil {
		// A reader who went away is not a failure, unless cleaning up
		// after them failed.
		var cleanup *usecase.CleanupError
		if r.Context().Err() == nil || errors.As(err, &cleanup) {
			slog.Error("character reply failed", "conversation", s.conversationID, "err", err)
		}
		_ = s.stream.Send(converters.ErrorEvent(err))
		return
	}
	_ = s.stream.Send(converters.DoneEvent(m))
}
