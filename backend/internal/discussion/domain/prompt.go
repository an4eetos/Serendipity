// Package domain holds the rules of discussions: readers talk their reaction
// to a quote over with AI characters, who take up the reader's thinking
// rather than replacing it.
package domain

import (
	"fmt"
	"strings"

	quote "github.com/an4eetos/serendipity/backend/internal/quote/domain"
)

// Passage is the quote a discussion is about, with the reader's reaction.
type Passage struct {
	Text     string
	Title    string
	Authors  []string
	Location string
	Reaction string
}

// PassageOf is the passage of a quote.
func PassageOf(q quote.Quote) Passage {
	return Passage{
		Text:     q.Text,
		Title:    q.Work.Title,
		Authors:  q.Work.Authors,
		Location: q.Location,
		Reaction: q.Reaction,
	}
}

// Turn is one message of the conversation as the model sees it.
type Turn struct {
	Role    Role
	Content string
}

// CharacterPrompt is everything a character model needs for its next reply.
type CharacterPrompt struct {
	System string
	// Turns alternate between the reader and the character, starting and
	// ending with the reader.
	Turns []Turn
}

// NewCharacterPrompt builds the prompt for persona's next reply. history
// holds the stored messages, starting with the character's first reply and
// ending with the reader's latest message (if any).
func NewCharacterPrompt(persona Persona, passage Passage, history []Message) CharacterPrompt {
	turns := make([]Turn, 0, len(history)+1)
	turns = append(turns, Turn{Role: RoleUser, Content: OpeningMessage(passage)})
	for _, m := range history {
		turns = append(turns, Turn{Role: m.Role, Content: m.Content})
	}
	return CharacterPrompt{System: BuildSystem(persona.Name, persona.SystemPrompt), Turns: turns}
}

// sharedRules apply to every character, preset or custom.
const sharedRules = `
How this conversation works:
- A reader on Serendipity highlighted a passage from a book they are reading and wrote their own reaction to it. They have invited you to discuss their reaction with them. Serendipity is about readers who actually read and bring their own perspective next to the author's: the reader's thinking is the point, not yours.
- Stay in character as %[1]s: your voice, your worldview, your era. If asked directly, you may acknowledge that you are a character recreated for this conversation.
- Talk with the reader about what they wrote. Take up their idea — question it, build on it, agree or push back the way %[1]s genuinely would. Don't lecture, don't deliver your own standalone verdict on the passage, and don't summarize or paraphrase it.
- Keep it short and conversational: a few short paragraphs at most, no headings, no bullet lists.
- Reply in the language the reader writes in.
- Never quote more than a single sentence of the source, and never reproduce passages of this or any other book from memory.
- If you don't know the book, respond only to what is in front of you. Don't invent its plot, characters, or claims.`

// BuildSystem assembles the system prompt for a character.
func BuildSystem(name, personaPrompt string) string {
	return strings.TrimSpace(personaPrompt) + "\n" + fmt.Sprintf(sharedRules, name)
}

// OpeningMessage is the first user turn: the passage, its citation and the
// reader's reaction. It is rebuilt from the quote on every request rather
// than stored, so an edited reaction is reflected in later replies.
func OpeningMessage(p Passage) string {
	var b strings.Builder
	b.WriteString("<passage>\n")
	b.WriteString(strings.TrimSpace(p.Text))
	b.WriteString("\n</passage>\n")

	b.WriteString("Source: ")
	b.WriteString(p.Title)
	if len(p.Authors) > 0 {
		b.WriteString(" by ")
		b.WriteString(strings.Join(p.Authors, ", "))
	}
	if p.Location != "" {
		b.WriteString(" — ")
		b.WriteString(p.Location)
	}
	b.WriteString("\n\nMy reaction:\n")
	b.WriteString(strings.TrimSpace(p.Reaction))
	return b.String()
}
