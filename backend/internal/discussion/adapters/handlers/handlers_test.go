package handlers

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/an4eetos/serendipity/backend/internal/discussion/domain"
	"github.com/an4eetos/serendipity/backend/internal/discussion/ports"
	"github.com/an4eetos/serendipity/backend/internal/discussion/usecase"
	"github.com/an4eetos/serendipity/backend/internal/kernel/adapters/httpapi"
	"github.com/an4eetos/serendipity/backend/internal/kernel/ports/portstest"
	quote "github.com/an4eetos/serendipity/backend/internal/quote/domain"
	quoteports "github.com/an4eetos/serendipity/backend/internal/quote/ports"
)

type fakeQuotes struct{ quoteports.QuoteRepository }

func (fakeQuotes) Get(_ context.Context, _, id string) (quote.Quote, error) {
	q := quote.Quote{ID: id, Text: "Know thyself."}
	if id == "reacted" {
		q.Reaction = "Easier said."
	}
	return q, nil
}

type fakePersonas struct{ ports.PersonaRepository }

func (fakePersonas) Get(_ context.Context, _, id string) (domain.Persona, error) {
	return domain.Persona{ID: id, Name: "Diogenes"}, nil
}

type fakeConversations struct{ ports.ConversationRepository }

func (fakeConversations) Create(context.Context, string, string, string) (string, error) {
	return "conv-1", nil
}

func (fakeConversations) Delete(context.Context, string, string) error { return nil }

func (fakeConversations) AppendMessage(_ context.Context, _ string, role domain.Role, content string) (domain.Message, error) {
	return domain.Message{ID: 7, Role: role, Content: content}, nil
}

type fakeModel struct{ err error }

func (m fakeModel) Reply(_ context.Context, _ domain.CharacterPrompt, onText func(string) error) (string, error) {
	if m.err != nil {
		return "", m.err
	}
	for _, c := range []string{"Hel", "lo"} {
		if err := onText(c); err != nil {
			return "", err
		}
	}
	return "Hello", nil
}

func serve(t *testing.T, model ports.CharacterModel, body string) *httptest.ResponseRecorder {
	t.Helper()
	start := usecase.NewStartConversation(&portstest.TxManager{}, fakeQuotes{}, fakePersonas{}, fakeConversations{}, model)
	r := chi.NewRouter()
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			next.ServeHTTP(w, r.WithContext(httpapi.WithUserID(r.Context(), "u1")))
		})
	})
	New(nil, nil, start, nil).RegisterRoutes(r, r)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/conversations", strings.NewReader(body)))
	return w
}

func TestCreateConversationStreams(t *testing.T) {
	w := serve(t, fakeModel{}, `{"quote_id":"reacted","persona_id":"p1"}`)
	if w.Code != http.StatusOK || w.Header().Get("Content-Type") != "text/event-stream" {
		t.Fatalf("status %d, Content-Type %q", w.Code, w.Header().Get("Content-Type"))
	}
	want := `data: {"type":"start","conversation_id":"conv-1"}` + "\n\n" +
		`data: {"type":"delta","text":"Hel"}` + "\n\n" +
		`data: {"type":"delta","text":"lo"}` + "\n\n" +
		`data: {"type":"done","message":{"id":7,"role":"assistant","content":"Hello","created_at":"0001-01-01T00:00:00Z"}}` + "\n\n"
	if got := w.Body.String(); got != want {
		t.Errorf("body =\n%s\nwant\n%s", got, want)
	}
}

func TestCreateConversationFailures(t *testing.T) {
	tests := []struct {
		name       string
		model      fakeModel
		body       string
		wantStatus int
		wantBody   string
	}{
		{
			name:       "rejected before streaming",
			body:       `{"quote_id":"bare","persona_id":"p1"}`,
			wantStatus: http.StatusBadRequest,
			wantBody:   `{"error":"write your reaction first"}` + "\n",
		},
		{
			name:       "unknown field",
			body:       `{"quote_id":"reacted","persona":"p1"}`,
			wantStatus: http.StatusBadRequest,
			wantBody:   `{"error":"invalid JSON body: json: unknown field \"persona\""}` + "\n",
		},
		{
			name:       "model down after streaming started",
			model:      fakeModel{err: errors.New("overloaded")},
			body:       `{"quote_id":"reacted","persona_id":"p1"}`,
			wantStatus: http.StatusOK,
			wantBody: `data: {"type":"start","conversation_id":"conv-1"}` + "\n\n" +
				`data: {"type":"error","error":"the character could not answer right now — try again"}` + "\n\n",
		},
		{
			name:       "refusal",
			model:      fakeModel{err: domain.ErrRefused},
			body:       `{"quote_id":"reacted","persona_id":"p1"}`,
			wantStatus: http.StatusOK,
			wantBody: `data: {"type":"start","conversation_id":"conv-1"}` + "\n\n" +
				`data: {"type":"error","error":"the character declined to respond to this"}` + "\n\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := serve(t, tt.model, tt.body)
			if w.Code != tt.wantStatus || w.Body.String() != tt.wantBody {
				t.Errorf("got %d %q\nwant %d %q", w.Code, w.Body.String(), tt.wantStatus, tt.wantBody)
			}
		})
	}
}
