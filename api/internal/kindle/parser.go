// Package kindle parses the "My Clippings.txt" file that Kindle devices keep.
//
// The file is a sequence of entries separated by "==========" lines:
//
//	The Republic (Plato)
//	- Your Highlight on page 12 | Location 170-172 | Added on Monday, 3 March 2025 10:00:00
//
//	The highlighted text...
//	==========
//
// Highlights are grouped per book, notes are attached to the highlight they
// were written on, bookmarks are dropped, and highlights that Kindle kept
// twice because the reader extended them are collapsed into the latest one.
package kindle

import (
	"bufio"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"
)

const separator = "=========="

// Book groups the highlights of one title.
type Book struct {
	Title      string
	Author     string
	Highlights []Highlight
	// OrphanNotes counts notes that could not be matched to a highlight.
	OrphanNotes int
}

// Highlight is one highlighted passage, with the reader's note if any.
type Highlight struct {
	Text     string
	Note     string
	Page     string
	LocStart int // 0 when unknown
	LocEnd   int // 0 when unknown
}

// Location renders a human-readable citation such as "Page 12 · Loc 170-172".
func (h Highlight) Location() string {
	var parts []string
	if h.Page != "" {
		parts = append(parts, "Page "+h.Page)
	}
	switch {
	case h.LocStart > 0 && h.LocEnd > h.LocStart:
		parts = append(parts, fmt.Sprintf("Loc %d-%d", h.LocStart, h.LocEnd))
	case h.LocStart > 0:
		parts = append(parts, fmt.Sprintf("Loc %d", h.LocStart))
	}
	return strings.Join(parts, " · ")
}

type kind int

const (
	kindHighlight kind = iota
	kindNote
	kindBookmark
)

type clipping struct {
	kind     kind
	text     string
	page     string
	locStart int
	locEnd   int
}

var (
	pageRe     = regexp.MustCompile(`(?i)\bpage\s+([0-9ivxlcdm]+(?:-[0-9ivxlcdm]+)?)`)
	locationRe = regexp.MustCompile(`(?i)\b(?:location|loc\.?|position)\s+(\d+)(?:-(\d+))?`)
)

// Parse reads a My Clippings.txt file.
func Parse(r io.Reader) ([]Book, error) {
	var (
		books   []*Book
		byKey   = map[string]*Book{}
		pending = map[*Book][]clipping{} // notes, attached once all highlights are known
		entry   []string
	)

	flush := func() {
		lines := entry
		entry = nil
		// Drop blank lines before the title.
		for len(lines) > 0 && strings.TrimSpace(lines[0]) == "" {
			lines = lines[1:]
		}
		if len(lines) < 2 {
			return
		}
		title, author := splitTitleAuthor(strings.TrimSpace(lines[0]))
		if title == "" {
			return
		}
		c := parseMeta(lines[1])
		c.text = strings.TrimSpace(strings.Join(lines[2:], "\n"))
		if c.kind == kindBookmark || c.text == "" || isClippingLimitMessage(c.text) {
			return
		}

		key := strings.ToLower(title) + "\x00" + strings.ToLower(author)
		b, ok := byKey[key]
		if !ok {
			b = &Book{Title: title, Author: author}
			byKey[key] = b
			books = append(books, b)
		}
		if c.kind == kindNote {
			pending[b] = append(pending[b], c)
			return
		}
		addHighlight(b, Highlight{Text: c.text, Page: c.page, LocStart: c.locStart, LocEnd: c.locEnd})
	}

	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 64*1024), 4*1024*1024)
	for sc.Scan() {
		line := strings.TrimRight(strings.ReplaceAll(sc.Text(), "\ufeff", ""), "\r")
		if strings.TrimSpace(line) == separator {
			flush()
			continue
		}
		entry = append(entry, line)
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("read clippings: %w", err)
	}
	flush()

	out := make([]Book, 0, len(books))
	for _, b := range books {
		for _, n := range pending[b] {
			if !attachNote(b, n) {
				b.OrphanNotes++
			}
		}
		if len(b.Highlights) > 0 {
			out = append(out, *b)
		}
	}
	return out, nil
}

// splitTitleAuthor splits "Title (Author)" on the last balanced parenthesis
// group, so titles like "Meditations (Penguin Classics) (Marcus Aurelius)"
// keep their own parentheses.
func splitTitleAuthor(line string) (title, author string) {
	if !strings.HasSuffix(line, ")") {
		return line, ""
	}
	depth := 0
	for i := len(line) - 1; i >= 0; i-- {
		switch line[i] {
		case ')':
			depth++
		case '(':
			depth--
			if depth == 0 {
				title = strings.TrimSpace(line[:i])
				author = strings.TrimSpace(line[i+1 : len(line)-1])
				if title == "" {
					return line, ""
				}
				return title, author
			}
		}
	}
	return line, ""
}

func parseMeta(meta string) clipping {
	// The kind is named before the first "|"; the rest holds the date, whose
	// month names must not be mistaken for a kind.
	head, _, _ := strings.Cut(meta, "|")
	lower := strings.ToLower(head)
	c := clipping{kind: kindHighlight}
	switch {
	case strings.Contains(lower, "bookmark") || strings.Contains(lower, "lesezeichen") || strings.Contains(lower, "signet"):
		c.kind = kindBookmark
	case strings.Contains(lower, "note") || strings.Contains(lower, "notiz"):
		c.kind = kindNote
	}
	if m := pageRe.FindStringSubmatch(meta); m != nil {
		c.page = m[1]
	}
	if m := locationRe.FindStringSubmatch(meta); m != nil {
		c.locStart, _ = strconv.Atoi(m[1])
		c.locEnd = c.locStart
		if m[2] != "" {
			c.locEnd, _ = strconv.Atoi(m[2])
		}
	}
	return c
}

func isClippingLimitMessage(text string) bool {
	return strings.Contains(strings.ToLower(text), "clipping limit")
}

// addHighlight appends h, replacing an earlier version of the same highlight
// (identical text, or one text containing the other at an overlapping spot).
func addHighlight(b *Book, h Highlight) {
	for i, old := range b.Highlights {
		if old.Text == h.Text {
			return
		}
		if overlaps(old, h) && (strings.Contains(h.Text, old.Text) || strings.Contains(old.Text, h.Text)) {
			b.Highlights[i] = h
			return
		}
	}
	b.Highlights = append(b.Highlights, h)
}

func overlaps(a, b Highlight) bool {
	if a.LocStart > 0 && b.LocStart > 0 {
		return a.LocStart <= b.LocEnd && b.LocStart <= a.LocEnd
	}
	return a.Page != "" && a.Page == b.Page
}

// attachNote adds the note to the last highlight covering its location.
func attachNote(b *Book, n clipping) bool {
	for i := len(b.Highlights) - 1; i >= 0; i-- {
		h := &b.Highlights[i]
		var match bool
		if n.locStart > 0 && h.LocStart > 0 {
			match = n.locStart >= h.LocStart && n.locStart <= h.LocEnd
		} else {
			match = n.page != "" && n.page == h.Page
		}
		if !match {
			continue
		}
		if h.Note != "" {
			h.Note += "\n\n"
		}
		h.Note += n.text
		return true
	}
	return false
}
