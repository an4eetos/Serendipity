// Package llm runs the AI characters readers discuss their reactions with.
package llm

import (
	"fmt"
	"strings"
)

// Passage is the quote a discussion is about, with the reader's reaction.
type Passage struct {
	Text     string
	Title    string
	Authors  []string
	Location string
	Reaction string
}

// Turn is one stored message of a discussion.
type Turn struct {
	Role    string // "user" or "assistant"
	Content string
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
