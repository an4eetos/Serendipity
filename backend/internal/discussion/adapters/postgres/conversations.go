package postgres

import (
	"context"

	"github.com/an4eetos/serendipity/backend/internal/discussion/domain"
	kernelpg "github.com/an4eetos/serendipity/backend/internal/kernel/adapters/postgres"
	kernel "github.com/an4eetos/serendipity/backend/internal/kernel/domain"
)

// ConversationRepository implements ports.ConversationRepository.
type ConversationRepository struct {
	db *kernelpg.DB
}

func NewConversationRepository(db *kernelpg.DB) *ConversationRepository {
	return &ConversationRepository{db: db}
}

func (r *ConversationRepository) Create(ctx context.Context, userID, quoteID, personaID string) (string, error) {
	var id string
	err := r.db.QueryRow(ctx, `
		insert into conversations (user_id, quote_id, persona_id)
		values ($1, $2, $3) returning id`, userID, quoteID, personaID).Scan(&id)
	return id, err
}

func (r *ConversationRepository) Delete(ctx context.Context, userID, id string) error {
	_, err := r.db.Exec(ctx, `delete from conversations where user_id = $1 and id = $2`, userID, id)
	return err
}

func (r *ConversationRepository) AppendMessage(ctx context.Context, conversationID string, role domain.Role, content string) (domain.Message, error) {
	m := domain.Message{Role: role, Content: content}
	err := r.db.QueryRow(ctx, `
		insert into messages (conversation_id, role, content) values ($1, $2::message_role, $3)
		returning id, created_at`, conversationID, role, content).Scan(&m.ID, &m.CreatedAt)
	return m, err
}

func (r *ConversationRepository) DeleteMessage(ctx context.Context, conversationID string, id int64) error {
	_, err := r.db.Exec(ctx, `delete from messages where conversation_id = $1 and id = $2`, conversationID, id)
	return err
}

func (r *ConversationRepository) Get(ctx context.Context, userID, id string) (domain.Conversation, error) {
	convs, err := r.list(ctx, `c.user_id = $1 and c.id = $2`, userID, id)
	if err != nil {
		return domain.Conversation{}, err
	}
	if len(convs) == 0 {
		return domain.Conversation{}, kernel.ErrNotFound
	}
	return convs[0], nil
}

func (r *ConversationRepository) ListForQuote(ctx context.Context, userID, quoteID string) ([]domain.Conversation, error) {
	return r.list(ctx, `c.user_id = $1 and c.quote_id = $2`, userID, quoteID)
}

func (r *ConversationRepository) ListForQuotes(ctx context.Context, userID string, quoteIDs []string) ([]domain.Conversation, error) {
	return r.list(ctx, `c.user_id = $1 and c.quote_id = any($2::uuid[])`, userID, quoteIDs)
}

// list returns the conversations matching where, oldest first, with their
// personas and messages.
func (r *ConversationRepository) list(ctx context.Context, where string, args ...any) ([]domain.Conversation, error) {
	rows, err := r.db.Query(ctx, `
		select c.id, c.quote_id, c.created_at, `+personaColumns+`
		from conversations c join personas p on p.id = c.persona_id
		where `+where+` order by c.created_at`, args...)
	if err != nil {
		return nil, err
	}
	var convs []domain.Conversation
	ids := []string{}
	for rows.Next() {
		var c domain.Conversation
		dest := append([]any{&c.ID, &c.QuoteID, &c.CreatedAt}, personaDest(&c.Persona)...)
		if err := rows.Scan(dest...); err != nil {
			rows.Close()
			return nil, err
		}
		c.Messages = []domain.Message{}
		convs = append(convs, c)
		ids = append(ids, c.ID)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(convs) == 0 {
		return []domain.Conversation{}, nil
	}
	return convs, r.attachMessages(ctx, convs, ids)
}

func (r *ConversationRepository) attachMessages(ctx context.Context, convs []domain.Conversation, ids []string) error {
	byID := make(map[string]*domain.Conversation, len(convs))
	for i := range convs {
		byID[convs[i].ID] = &convs[i]
	}
	rows, err := r.db.Query(ctx, `
		select conversation_id, id, role, content, created_at from messages
		where conversation_id = any($1::uuid[]) order by id`, ids)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var cid string
		var m domain.Message
		if err := rows.Scan(&cid, &m.ID, &m.Role, &m.Content, &m.CreatedAt); err != nil {
			return err
		}
		if c := byID[cid]; c != nil {
			c.Messages = append(c.Messages, m)
		}
	}
	return rows.Err()
}
