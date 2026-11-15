//go:build e2e

package e2e

import (
	"net/http"
	"slices"
	"strings"
	"testing"
)

const (
	gatedMsg     = "accept the current terms of service first"
	ownershipMsg = "confirm that you own this copy first"
	// q1Text and q2Text are 8 distinct words each: one fits the cap of 12,
	// both don't.
	q1Text     = "Quiet rivers carry ancient stones toward patient seas"
	q2Text     = "Bright lanterns guide weary travelers across frozen hills"
	kindleText = "The beginning is the most important part of the work."
)

// stepTerms: no write before accepting the current terms; accepting an old
// version is a conflict.
func (s *scenario) stepTerms(t *testing.T) {
	a := s.a
	t.Run("anonymous GET terms returns the version", func(t *testing.T) {
		got := expectDecode[versionBody](t, s.api.call(t, http.MethodGet, "/terms", "", nil), http.StatusOK)
		if got.Version != s.terms {
			t.Fatalf("version %q, want %q", got.Version, s.terms)
		}
	})
	t.Run("GET me before accepting has terms_accepted false", func(t *testing.T) {
		me := expectDecode[meBody](t, s.api.call(t, http.MethodGet, "/me", a.token, nil), http.StatusOK)
		if me.TermsAccepted || me.CurrentTermsVersion != s.terms {
			t.Fatalf("terms_accepted %v, current_terms_version %q", me.TermsAccepted, me.CurrentTermsVersion)
		}
	})
	t.Run("POST library before accepting is 403", func(t *testing.T) {
		expectError(t, s.api.call(t, http.MethodPost, "/library", a.token, s.bookA()), http.StatusForbidden, gatedMsg)
	})
	t.Run("accepting an old version is 409", func(t *testing.T) {
		r := s.api.call(t, http.MethodPost, "/me/terms", a.token, map[string]string{"version": "draft-2026-10"})
		expectError(t, r, http.StatusConflict, "those terms are not the current version")
	})
	t.Run("accepting the current version is 200", func(t *testing.T) {
		s.acceptTerms(t, a)
	})
	t.Run("POST library after accepting is 201", func(t *testing.T) {
		item := expectDecode[libraryItemBody](t,
			s.api.call(t, http.MethodPost, "/library", a.token, s.bookA()), http.StatusCreated)
		s.itemA, s.workA = item.ID, item.Work.ID
		s.works = append(s.works, item.Work.ID)
		if item.OwnershipConfirmedAt != nil {
			t.Errorf("new item has ownership_confirmed_at %s", *item.OwnershipConfirmedAt)
		}
	})
	t.Run("reader B accepts the current version", func(t *testing.T) {
		s.acceptTerms(t, s.b)
	})
}

func (s *scenario) acceptTerms(t *testing.T, r reader) {
	t.Helper()
	me := expectDecode[meBody](t,
		s.api.call(t, http.MethodPost, "/me/terms", r.token, map[string]string{"version": s.terms}), http.StatusOK)
	if !me.TermsAccepted || me.TermsVersion != s.terms || me.TermsAcceptedAt == nil {
		t.Fatalf("after accepting: terms_accepted %v, terms_version %q", me.TermsAccepted, me.TermsVersion)
	}
}

// bookA is the tagged work that readers A and B share.
func (s *scenario) bookA() map[string]any {
	return map[string]any{
		"title": s.tag + " The Republic", "authors": []string{"Plato"},
		"openlibrary_key": "/works/" + s.tag, "subjects": []string{"Philosophy"},
	}
}

// stepOwnership: a file needs the ownership confirmation, which stamps
// ownership_confirmed_at; a confirmation without a file stamps nothing
// (CR-004); a Kindle import needs it and stamps the imported book.
func (s *scenario) stepOwnership(t *testing.T) {
	a := s.a
	t.Run("CR-004 confirmation without a file leaves ownership_confirmed_at null", func(t *testing.T) {
		need(t, s.itemA)
		item := expectDecode[libraryItemBody](t, s.api.call(t, http.MethodPatch, "/library/"+s.itemA, a.token,
			map[string]any{"ownership_confirmed": true}), http.StatusOK)
		if item.OwnershipConfirmedAt != nil {
			t.Fatalf("ownership_confirmed_at = %s, want null", *item.OwnershipConfirmedAt)
		}
	})
	t.Run("PATCH library with a file and no confirmation is 400", func(t *testing.T) {
		need(t, s.itemA)
		r := s.api.call(t, http.MethodPatch, "/library/"+s.itemA, a.token, s.fileBody(false))
		expectError(t, r, http.StatusBadRequest, ownershipMsg)
	})
	t.Run("PATCH library with a file and the confirmation stamps it", func(t *testing.T) {
		need(t, s.itemA)
		item := expectDecode[libraryItemBody](t,
			s.api.call(t, http.MethodPatch, "/library/"+s.itemA, a.token, s.fileBody(true)), http.StatusOK)
		if item.OwnershipConfirmedAt == nil {
			t.Error("ownership_confirmed_at is null")
		}
		if item.FilePath == nil || *item.FilePath != s.filePath() {
			t.Errorf("file_path %v, want %s", item.FilePath, s.filePath())
		}
	})
	t.Run("Kindle import without the confirmation is 400", func(t *testing.T) {
		expectError(t, s.api.importKindle(t, a.token, s.clippings(), false), http.StatusBadRequest, ownershipMsg)
	})
	t.Run("Kindle import with the confirmation imports 1 book and 1 quote", func(t *testing.T) {
		rep := expectDecode[importReportBody](t, s.api.importKindle(t, a.token, s.clippings(), true), http.StatusOK)
		if rep.Books != 1 || rep.Imported != 1 {
			t.Fatalf("report %+v, want 1 book and 1 imported", rep)
		}
	})
	t.Run("Kindle import stamps the imported book", func(t *testing.T) {
		need(t, s.workA)
		items := expectDecode[[]libraryItemBody](t, s.api.call(t, http.MethodGet, "/library", a.token, nil), http.StatusOK)
		i := slices.IndexFunc(items, func(it libraryItemBody) bool { return it.Work.ID != s.workA })
		if len(items) != 2 || i < 0 {
			t.Fatalf("library has %d items, want workA and the Kindle book", len(items))
		}
		s.kindleWork = items[i].Work.ID
		s.works = append(s.works, s.kindleWork)
		if items[i].OwnershipConfirmedAt == nil {
			t.Error("the imported book's ownership_confirmed_at is null")
		}
		qs := expectDecode[[]quoteBody](t,
			s.api.call(t, http.MethodGet, "/quotes?work_id="+s.kindleWork, a.token, nil), http.StatusOK)
		if len(qs) != 1 || qs[0].Source != "kindle" {
			t.Fatalf("Kindle work has %d quotes, want 1 from kindle", len(qs))
		}
		s.kindleQuote = qs[0].ID
	})
}

// filePath is A's upload path for itemA.
func (s *scenario) filePath() string {
	return s.a.id + "/" + s.itemA + ".epub"
}

func (s *scenario) fileBody(confirm bool) map[string]any {
	body := map[string]any{"file_path": s.filePath(), "file_type": "epub"}
	if confirm {
		body["ownership_confirmed"] = true
	}
	return body
}

// clippings is a My Clippings.txt with one tagged book and one highlight.
func (s *scenario) clippings() string {
	return s.tag + " Meditations (Marcus Aurelius)\n" +
		"- Your Highlight on page 12 | Location 170-172 | Added on Monday, 3 March 2025 10:00:00\n\n" +
		kindleText + "\n==========\n"
}

// stepStorage: readers write and list only their own folder of the books
// bucket, through the real Storage API.
func (s *scenario) stepStorage(t *testing.T) {
	a, b := s.a, s.b
	t.Run("upload into the own folder is 200", func(t *testing.T) {
		need(t, s.itemA)
		s.objectPath = s.filePath()
		s.objects = append(s.objects, s.objectPath)
		status, raw, err := s.sb.upload(t.Context(), a.token, s.objectPath, []byte("PK e2e epub"))
		if err != nil || status != http.StatusOK {
			t.Fatalf("upload: HTTP %d %s %v", status, snippet(raw), err)
		}
	})
	t.Run("upload into another reader's folder is refused", func(t *testing.T) {
		path := b.id + "/" + s.tag + ".epub"
		s.objects = append(s.objects, path)
		status, raw, err := s.sb.upload(t.Context(), a.token, path, []byte("PK e2e epub"))
		if err != nil || status < 400 {
			t.Fatalf("upload into B's folder: HTTP %d %s %v", status, snippet(raw), err)
		}
	})
	t.Run("listing another reader's folder is empty", func(t *testing.T) {
		need(t, s.objectPath)
		own, err := s.sb.list(t.Context(), a.token, a.id)
		if err != nil || len(own) != 1 {
			t.Fatalf("A lists its own folder: %v %v, want 1 object", own, err)
		}
		other, err := s.sb.list(t.Context(), b.token, a.id)
		if err != nil || len(other) != 0 {
			t.Fatalf("B lists A's folder: %v %v, want nothing", other, err)
		}
	})
}

// stepCap: the server decides passage_hidden; past the cap a passage is
// published collapsed; an identical passage (any case) counts once.
func (s *scenario) stepCap(t *testing.T) {
	a, b := s.a, s.b
	t.Run("A saves two private quotes", func(t *testing.T) {
		need(t, s.itemA)
		s.q1 = s.saveQuote(t, a, s.itemA, q1Text, 0.6)
		s.q2 = s.saveQuote(t, a, s.itemA, q2Text, 0.2)
	})
	t.Run("passage_hidden in a PATCH body is 400", func(t *testing.T) {
		need(t, s.q1)
		r := s.api.call(t, http.MethodPatch, "/quotes/"+s.q1, a.token,
			map[string]any{"visibility": "public", "passage_hidden": false})
		expectError(t, r, http.StatusBadRequest, "invalid JSON body:")
		q := expectDecode[quoteBody](t, s.api.call(t, http.MethodGet, "/quotes/"+s.q1, a.token, nil), http.StatusOK)
		if q.Visibility != "private" {
			t.Fatalf("the refused patch changed visibility to %s", q.Visibility)
		}
	})
	t.Run("8 words within the cap are shown", func(t *testing.T) {
		need(t, s.q1)
		if q := s.publish(t, a, s.q1); q.PassageHidden {
			t.Fatal("passage_hidden is true, want shown")
		}
	})
	t.Run("8 more distinct words are public and collapsed", func(t *testing.T) {
		need(t, s.q2)
		q := s.publish(t, a, s.q2)
		if !q.PassageHidden || q.Text != q2Text {
			t.Fatalf("passage_hidden %v, owner text %q; want collapsed with the full text", q.PassageHidden, q.Text)
		}
	})
	t.Run("B's identical passage in another case is shown", func(t *testing.T) {
		need(t, s.workA)
		item := expectDecode[libraryItemBody](t,
			s.api.call(t, http.MethodPost, "/library", b.token, s.bookA()), http.StatusCreated)
		if item.Work.ID != s.workA {
			t.Fatalf("B's work %s, want the shared work %s", item.Work.ID, s.workA)
		}
		s.q3 = s.saveQuote(t, b, item.ID, strings.ToUpper(q1Text), 0.5)
		if q := s.publish(t, b, s.q3); q.PassageHidden {
			t.Fatal("passage_hidden is true, want shown: an identical passage counts once")
		}
	})
}

// saveQuote creates a private quote that is ready to publish.
func (s *scenario) saveQuote(t *testing.T, r reader, itemID, text string, position float64) string {
	t.Helper()
	body := map[string]any{
		"library_item_id": itemID, "text": text, "location": "p. 12",
		"position": position, "reaction": "e2e reaction",
	}
	q := expectDecode[quoteBody](t, s.api.call(t, http.MethodPost, "/quotes", r.token, body), http.StatusCreated)
	if q.Visibility != "private" || q.PassageHidden {
		t.Fatalf("new quote: visibility %s, passage_hidden %v", q.Visibility, q.PassageHidden)
	}
	return q.ID
}

// publish makes a quote public and checks that it is.
func (s *scenario) publish(t *testing.T, r reader, id string) quoteBody {
	t.Helper()
	q := expectDecode[quoteBody](t, s.api.call(t, http.MethodPatch, "/quotes/"+id, r.token,
		map[string]string{"visibility": "public"}), http.StatusOK)
	if q.Visibility != "public" {
		t.Fatalf("visibility %s after publishing", q.Visibility)
	}
	return q
}
