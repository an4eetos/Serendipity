package domain

// CoverageCap is the default number of words of one work that may be public
// (PUBLIC_WORD_CAP).
type CoverageCap int

// Coverage is how much of a work is public, without the quote being
// published. Only distinct passages that anyone can read count: public, not
// removed and not collapsed, with identical passages counted once.
type Coverage struct {
	// Words is the length of the distinct public passages.
	Words int
	// IdenticalShown reports whether another public quote already shows the
	// same passage, which then adds nothing to the coverage.
	IdenticalShown bool
	// CapOverride is the work's own cap, which replaces the default one; nil
	// when the work has none.
	CapOverride *int
}

// ShowsPassage reports whether q's passage stays shown when q is published:
// it is already shown by another quote, or it fits under the work's cap.
func (c Coverage) ShowsPassage(q Quote, defaultCap CoverageCap) bool {
	if c.IdenticalShown {
		return true
	}
	limit := int(defaultCap)
	if c.CapOverride != nil {
		limit = *c.CapOverride
	}
	return c.Words+q.PassageWords <= limit
}
