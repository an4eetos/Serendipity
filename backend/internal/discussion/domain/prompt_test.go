package domain

import (
	"reflect"
	"strings"
	"testing"
)

func TestBuildSystem(t *testing.T) {
	s := BuildSystem("Diogenes", "You are Diogenes.")
	if !strings.HasPrefix(s, "You are Diogenes.") {
		t.Errorf("system prompt should start with the persona prompt: %q", s[:40])
	}
	for _, want := range []string{
		"Stay in character as Diogenes",
		"discuss their reaction",
		"Never quote more than a single sentence",
	} {
		if !strings.Contains(s, want) {
			t.Errorf("system prompt missing %q", want)
		}
	}
}

func TestOpeningMessage(t *testing.T) {
	p := Passage{
		Text:     "  The unexamined life is not worth living.  ",
		Title:    "Apology",
		Authors:  []string{"Plato"},
		Location: "38a",
		Reaction: "Is that too harsh?",
	}
	got := OpeningMessage(p)
	want := "<passage>\nThe unexamined life is not worth living.\n</passage>\n" +
		"Source: Apology by Plato — 38a\n\n" +
		"My reaction:\nIs that too harsh?"
	if got != want {
		t.Errorf("OpeningMessage() =\n%s\nwant\n%s", got, want)
	}

	p.Authors, p.Location = nil, ""
	if got := OpeningMessage(p); !strings.Contains(got, "Source: Apology\n\nMy reaction:") {
		t.Errorf("OpeningMessage() without authors/location = %q", got)
	}
}

func TestNewCharacterPrompt(t *testing.T) {
	persona := Persona{Name: "Diogenes", SystemPrompt: "You are Diogenes."}
	passage := Passage{Text: "x", Title: "t", Reaction: "r"}
	p := NewCharacterPrompt(persona, passage, []Message{
		{Role: RoleAssistant, Content: "reply"},
		{Role: RoleUser, Content: "answer"},
	})
	if p.System != BuildSystem("Diogenes", "You are Diogenes.") {
		t.Errorf("System = %q", p.System)
	}
	want := []Turn{
		{Role: RoleUser, Content: OpeningMessage(passage)},
		{Role: RoleAssistant, Content: "reply"},
		{Role: RoleUser, Content: "answer"},
	}
	if !reflect.DeepEqual(p.Turns, want) {
		t.Errorf("Turns = %#v, want %#v", p.Turns, want)
	}
}
