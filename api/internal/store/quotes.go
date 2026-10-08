package store

import (
	"context"
)

const quoteCols = `q.id, q.work_id, q.library_item_id, q.text, q.location, q.anchor, q.position,
	q.note, q.source, q.visibility, q.created_at, q.updated_at, ` + workCols

const quoteFrom = ` from quotes q join works w on w.id = q.work_id `

func scanQuote(row interface{ Scan(...any) error }) (Quote, error) {
	var q Quote
	q.Work = &Work{}
	dest := []any{&q.ID, &q.WorkID, &q.LibraryItemID, &q.Text, &q.Location, &q.Anchor, &q.Position,
		&q.Note, &q.Source, &q.Visibility, &q.CreatedAt, &q.UpdatedAt}
	dest = append(dest, scanWorkInto(q.Work)...)
	err := row.Scan(dest...)
	return q, notFound(err)
}

type NewQuote struct {
	WorkID        string
	LibraryItemID *string
	Text          string
	Location      string
	Anchor        *string
	Position      *float64
	Note          string
	Source        string
}

func (s *Store) CreateQuote(ctx context.Context, userID string, n NewQuote) (Quote, error) {
	var id string
	err := s.pool.QueryRow(ctx, `
		insert into quotes (user_id, work_id, library_item_id, text, location, anchor, position, note, source)
		values ($1, $2, $3, $4, $5, $6, $7, $8, $9::quote_source)
		on conflict (user_id, work_id, md5(text)) do update set updated_at = quotes.updated_at
		returning id`,
		userID, n.WorkID, n.LibraryItemID, n.Text, n.Location, n.Anchor, n.Position, n.Note, n.Source).Scan(&id)
	if err != nil {
		return Quote{}, err
	}
	return s.GetQuote(ctx, userID, id)
}

// ImportQuote inserts a quote unless the same text already exists for the
// work, and reports whether it was inserted.
func (s *Store) ImportQuote(ctx context.Context, userID string, n NewQuote) (bool, error) {
	tag, err := s.pool.Exec(ctx, `
		insert into quotes (user_id, work_id, library_item_id, text, location, note, source)
		values ($1, $2, $3, $4, $5, $6, $7::quote_source)
		on conflict (user_id, work_id, md5(text)) do nothing`,
		userID, n.WorkID, n.LibraryItemID, n.Text, n.Location, n.Note, n.Source)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() == 1, nil
}

func (s *Store) GetQuote(ctx context.Context, userID, id string) (Quote, error) {
	return scanQuote(s.pool.QueryRow(ctx, `select `+quoteCols+quoteFrom+`where q.user_id = $1 and q.id = $2`, userID, id))
}

// ListQuotes lists the user's quotes, optionally for one work.
func (s *Store) ListQuotes(ctx context.Context, userID, workID string) ([]Quote, error) {
	rows, err := s.pool.Query(ctx, `select `+quoteCols+quoteFrom+`
		where q.user_id = $1 and ($2 = '' or q.work_id::text = $2)
		order by q.position nulls last, q.created_at`, userID, workID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Quote{}
	for rows.Next() {
		q, err := scanQuote(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, q)
	}
	return out, rows.Err()
}

type QuotePatch struct {
	Note       *string `json:"note"`
	Visibility *string `json:"visibility"`
}

func (s *Store) UpdateQuote(ctx context.Context, userID, id string, p QuotePatch) (Quote, error) {
	tag, err := s.pool.Exec(ctx, `
		update quotes set
			note       = coalesce($3, note),
			visibility = coalesce($4::visibility, visibility),
			updated_at = now()
		where user_id = $1 and id = $2`, userID, id, p.Note, p.Visibility)
	if err != nil {
		return Quote{}, err
	}
	if tag.RowsAffected() == 0 {
		return Quote{}, ErrNotFound
	}
	return s.GetQuote(ctx, userID, id)
}

func (s *Store) DeleteQuote(ctx context.Context, userID, id string) error {
	tag, err := s.pool.Exec(ctx, `delete from quotes where user_id = $1 and id = $2`, userID, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// ---------------------------------------------------------------------------
// Personas
// ---------------------------------------------------------------------------

const personaCols = `p.id, p.slug, p.name, p.era, p.short_bio, p.system_prompt, p.is_preset`

func personaDest(p *Persona) []any {
	return []any{&p.ID, &p.Slug, &p.Name, &p.Era, &p.ShortBio, &p.SystemPrompt, &p.IsPreset}
}

// ListPersonas returns the presets plus the user's own characters.
func (s *Store) ListPersonas(ctx context.Context, userID string) ([]Persona, error) {
	rows, err := s.pool.Query(ctx, `select `+personaCols+` from personas p
		where p.is_preset or p.owner_id = $1 order by p.is_preset desc, p.sort_order, p.name`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Persona{}
	for rows.Next() {
		var p Persona
		if err := rows.Scan(personaDest(&p)...); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (s *Store) GetPersona(ctx context.Context, userID, id string) (Persona, error) {
	var p Persona
	err := s.pool.QueryRow(ctx, `select `+personaCols+` from personas p
		where p.id = $2 and (p.is_preset or p.owner_id = $1)`, userID, id).Scan(personaDest(&p)...)
	return p, notFound(err)
}

// ---------------------------------------------------------------------------
// Conversations
// ---------------------------------------------------------------------------

func (s *Store) CreateConversation(ctx context.Context, userID, quoteID, personaID, mode string) (string, error) {
	var id string
	err := s.pool.QueryRow(ctx, `
		insert into conversations (user_id, quote_id, persona_id, mode)
		values ($1, $2, $3, $4::conversation_mode) returning id`, userID, quoteID, personaID, mode).Scan(&id)
	return id, err
}

func (s *Store) DeleteConversation(ctx context.Context, userID, id string) error {
	_, err := s.pool.Exec(ctx, `delete from conversations where user_id = $1 and id = $2`, userID, id)
	return err
}

func (s *Store) AppendMessage(ctx context.Context, conversationID, role, content string) (Message, error) {
	m := Message{Role: role, Content: content}
	err := s.pool.QueryRow(ctx, `
		insert into messages (conversation_id, role, content) values ($1, $2::message_role, $3)
		returning id, created_at`, conversationID, role, content).Scan(&m.ID, &m.CreatedAt)
	return m, err
}

func (s *Store) DeleteMessage(ctx context.Context, conversationID string, id int64) error {
	_, err := s.pool.Exec(ctx, `delete from messages where conversation_id = $1 and id = $2`, conversationID, id)
	return err
}

// GetConversation returns one of the user's conversations with its persona
// (including the system prompt) and messages.
func (s *Store) GetConversation(ctx context.Context, userID, id string) (Conversation, error) {
	convs, err := s.conversations(ctx, `c.user_id = $1 and c.id = $2`, userID, id)
	if err != nil {
		return Conversation{}, err
	}
	if len(convs) == 0 {
		return Conversation{}, ErrNotFound
	}
	return convs[0], nil
}

func (s *Store) ListConversationsForQuote(ctx context.Context, userID, quoteID string) ([]Conversation, error) {
	return s.conversations(ctx, `c.user_id = $1 and c.quote_id = $2`, userID, quoteID)
}

func (s *Store) conversations(ctx context.Context, where string, args ...any) ([]Conversation, error) {
	rows, err := s.pool.Query(ctx, `
		select c.id, c.quote_id, c.mode, c.created_at, `+personaCols+`
		from conversations c join personas p on p.id = c.persona_id
		where `+where+` order by c.created_at`, args...)
	if err != nil {
		return nil, err
	}
	var convs []Conversation
	ids := []string{}
	for rows.Next() {
		var c Conversation
		dest := append([]any{&c.ID, &c.QuoteID, &c.Mode, &c.CreatedAt}, personaDest(&c.Persona)...)
		if err := rows.Scan(dest...); err != nil {
			rows.Close()
			return nil, err
		}
		c.Messages = []Message{}
		convs = append(convs, c)
		ids = append(ids, c.ID)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(convs) == 0 {
		return []Conversation{}, nil
	}
	return convs, s.attachMessages(ctx, convs, ids)
}

func (s *Store) attachMessages(ctx context.Context, convs []Conversation, ids []string) error {
	byID := make(map[string]*Conversation, len(convs))
	for i := range convs {
		byID[convs[i].ID] = &convs[i]
	}
	rows, err := s.pool.Query(ctx, `
		select conversation_id, id, role, content, created_at from messages
		where conversation_id = any($1::uuid[]) order by id`, ids)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var cid string
		var m Message
		if err := rows.Scan(&cid, &m.ID, &m.Role, &m.Content, &m.CreatedAt); err != nil {
			return err
		}
		if c := byID[cid]; c != nil {
			c.Messages = append(c.Messages, m)
		}
	}
	return rows.Err()
}
