// Package llm runs the AI characters that react to and argue about quotes.
package llm

import (
	"fmt"
	"strings"
)

// Passage is the quote a conversation is about.
type Passage struct {
	Text     string
	Title    string
	Authors  []string
	Location string
	Note     string
}

// Turn is one stored message of a conversation.
type Turn struct {
	Role    string // "user" or "assistant"
	Content string
}

const (
	ModeReact = "react"
	ModeArgue = "argue"
)

// sharedRules apply to every character, preset or custom.
const sharedRules = `
How this conversation works:
- A reader on Serendipity has highlighted a passage from a book they are reading and brought it to you, sometimes with a note of their own. Serendipity is about readers who actually read, and who add a fresh perspective next to the author's.
- Stay in character as %[1]s: your voice, your worldview, your era. If asked directly, you may acknowledge that you are a character recreated for this conversation.
- Respond to the passage itself — what it stirs in you, where you agree or object, what it connects to in your own thought, the question it raises. Do not summarize or paraphrase it back.
- Take the reader's note seriously. Their perspective matters as much as the author's.
- Keep it short and conversational: a few short paragraphs at most, no headings, no bullet lists.
- Reply in the language the reader writes in; if they have written nothing, use the passage's language.
- Never quote more than a single sentence of the source, and never reproduce passages of this or any other book from memory.
- If you don't know the book, respond only to what is in front of you. Don't invent its plot, characters, or claims.`

var modeRules = map[string]string{
	ModeReact: `
Mode: reaction. Give %[1]s's honest reaction — agreement, delight, unease, amusement, disagreement, whatever %[1]s would genuinely feel. Invite the reader to answer only if it comes naturally.`,
	ModeArgue: `
Mode: argument. The reader wants to argue with you. Take a position against the passage or against the reader's note — whichever %[1]s would find more worth disputing — and press it in %[1]s's characteristic way. Be sharp but fair, and concede a point when the reader earns it.`,
}

// ValidMode reports whether mode is a supported conversation mode.
func ValidMode(mode string) bool {
	_, ok := modeRules[mode]
	return ok
}

// BuildSystem assembles the system prompt for a character in a mode.
func BuildSystem(name, personaPrompt, mode string) string {
	rules, ok := modeRules[mode]
	if !ok {
		rules = modeRules[ModeReact]
	}
	return strings.TrimSpace(personaPrompt) + "\n" + fmt.Sprintf(sharedRules, name) + "\n" + fmt.Sprintf(rules, name)
}

// OpeningMessage is the first user turn: the passage, its citation and the
// reader's note. It is rebuilt from the quote on every request rather than
// stored, so an edited note is reflected in later replies.
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
	b.WriteString("\n\n")

	if note := strings.TrimSpace(p.Note); note != "" {
		b.WriteString("The reader's note:\n")
		b.WriteString(note)
	} else {
		b.WriteString("The reader added no note.")
	}
	return b.String()
}
