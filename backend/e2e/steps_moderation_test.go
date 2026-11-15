//go:build e2e

package e2e

import (
	"net/http"
	"testing"
)

const (
	takedownReason = "e2e: reported as a copyright problem"
	adminsOnlyMsg  = "admins only"
)

// stepPublicProfile: a private profile shows nothing and its quotes can't be
// reported (CR-009); a public one lists the quotes in their public
// projection, the trails and the answered discussion.
func (s *scenario) stepPublicProfile(t *testing.T) {
	a := s.a
	t.Run("a private profile is 404 to anonymous readers", func(t *testing.T) {
		expect(t, s.api.call(t, http.MethodGet, "/profiles/"+a.username, "", nil), http.StatusNotFound)
	})
	t.Run("CR-009 reporting a quote on a private profile is 404", func(t *testing.T) {
		need(t, s.q1)
		r := s.api.call(t, http.MethodPost, "/reports", s.b.token, reportReq("quote", s.q1))
		if r.status == http.StatusCreated {
			// Undo it, so the reports step still starts from no report.
			if err := deleteReport(t.Context(), s.db, decode[reportBody](t, r).ID); err != nil {
				t.Errorf("delete the stray report: %v", err)
			}
		}
		expectError(t, r, http.StatusNotFound, "not found")
	})
	t.Run("A makes the profile public", func(t *testing.T) {
		me := expectDecode[meBody](t, s.api.call(t, http.MethodPatch, "/me", a.token,
			map[string]bool{"is_public": true}), http.StatusOK)
		if !me.IsPublic {
			t.Fatal("is_public is false after the patch")
		}
	})
	t.Run("A publishes the Kindle quote", func(t *testing.T) {
		need(t, s.kindleQuote)
		q := expectDecode[quoteBody](t, s.api.call(t, http.MethodPatch, "/quotes/"+s.kindleQuote, a.token,
			map[string]string{"reaction": "e2e kindle reaction", "visibility": "public"}), http.StatusOK)
		if q.Visibility != "public" || q.Position != nil {
			t.Fatalf("visibility %s, position %v; want public without a position", q.Visibility, q.Position)
		}
	})
	t.Run("an answered discussion of q1 is stored with SQL", func(t *testing.T) {
		need(t, s.q1)
		id, err := insertAnsweredConversation(t.Context(), s.db, a.id, s.q1)
		if err != nil {
			t.Fatal(err)
		}
		s.conv = id
	})
	s.checkProfileContents(t)
}

// checkProfileContents: the public projection of A's quotes, the trails and
// the answered discussion.
func (s *scenario) checkProfileContents(t *testing.T) {
	t.Run("both quotes are listed and the collapsed one is blanked", func(t *testing.T) {
		need(t, s.q1, s.q2)
		p := s.publicProfile(t)
		q1, q2 := p.quote(s.q1), p.quote(s.q2)
		if q1 == nil || q2 == nil {
			t.Fatalf("q1 listed %v, q2 listed %v", q1 != nil, q2 != nil)
		}
		if q1.Text != q1Text || q1.PassageHidden {
			t.Errorf("q1 text %q, passage_hidden %v; want the passage shown", q1.Text, q1.PassageHidden)
		}
		if q2.Text != "" || !q2.PassageHidden || q2.Reaction == "" || q2.Location == "" {
			t.Errorf("q2 text %q, passage_hidden %v, reaction %q, location %q; want blank text, reaction and citation",
				q2.Text, q2.PassageHidden, q2.Reaction, q2.Location)
		}
	})
	t.Run("the trail has 2 points in position order", func(t *testing.T) {
		need(t, s.workA, s.q1, s.q2)
		book := s.publicProfile(t).book(s.workA)
		if book == nil {
			t.Fatal("the shared work is not among the books")
		}
		pts := book.Trail.Points
		if len(pts) != 2 || pts[0].QuoteID != s.q2 || pts[1].QuoteID != s.q1 || book.Trail.Unplaced != 0 {
			t.Fatalf("trail %+v, want q2 (0.2) then q1 (0.6) and 0 unplaced", book.Trail)
		}
		if book.PublicQuotes != 2 {
			t.Errorf("public_quotes %d, want 2", book.PublicQuotes)
		}
	})
	t.Run("the published Kindle quote is unplaced", func(t *testing.T) {
		need(t, s.kindleWork)
		book := s.publicProfile(t).book(s.kindleWork)
		if book == nil || book.Trail.Points == nil || len(book.Trail.Points) != 0 || book.Trail.Unplaced != 1 {
			t.Fatalf("Kindle book %+v, want trail {\"points\": [], \"unplaced\": 1}", book)
		}
	})
	t.Run("the answered discussion is shown", func(t *testing.T) {
		need(t, s.q1, s.conv)
		q1 := s.publicProfile(t).quote(s.q1)
		if q1 == nil || len(q1.Conversations) != 1 || q1.Conversations[0].ID != s.conv {
			t.Fatalf("q1 %+v, want the discussion %s", q1, s.conv)
		}
	})
}

// publicProfile reads A's profile as an anonymous visitor.
func (s *scenario) publicProfile(t *testing.T) publicProfileBody {
	t.Helper()
	return expectDecode[publicProfileBody](t,
		s.api.call(t, http.MethodGet, "/profiles/"+s.a.username, "", nil), http.StatusOK)
}

func reportReq(kind, id string) map[string]string {
	return map[string]string{"target_kind": kind, "target_id": id, "reason": "copyright", "note": "e2e"}
}

// stepReports: one open report per reader and target; never on own content
// or on something that isn't public.
func (s *scenario) stepReports(t *testing.T) {
	b := s.b
	t.Run("reporting a quote is 201", func(t *testing.T) {
		need(t, s.q1)
		rep := expectDecode[reportBody](t,
			s.api.call(t, http.MethodPost, "/reports", b.token, reportReq("quote", s.q1)), http.StatusCreated)
		if rep.Status != "open" || rep.TargetID != s.q1 {
			t.Fatalf("report %+v, want open on q1", rep)
		}
	})
	t.Run("reporting it again is 409", func(t *testing.T) {
		need(t, s.q1)
		r := s.api.call(t, http.MethodPost, "/reports", b.token, reportReq("quote", s.q1))
		expectError(t, r, http.StatusConflict, "you already reported this")
	})
	t.Run("reporting own content is 400", func(t *testing.T) {
		need(t, s.q1)
		r := s.api.call(t, http.MethodPost, "/reports", s.a.token, reportReq("quote", s.q1))
		expectError(t, r, http.StatusBadRequest, "you can't report your own content")
	})
	t.Run("reporting a missing quote is 404", func(t *testing.T) {
		r := s.api.call(t, http.MethodPost, "/reports", b.token, reportReq("quote", newUUID(t)))
		expectError(t, r, http.StatusNotFound, "not found")
	})
	t.Run("reporting a discussion is 201", func(t *testing.T) {
		need(t, s.conv)
		rep := expectDecode[reportBody](t, s.api.call(t, http.MethodPost, "/reports", b.token,
			reportReq("conversation", s.conv)), http.StatusCreated)
		if rep.TargetKind != "conversation" || rep.TargetID != s.conv {
			t.Fatalf("report %+v, want one on the discussion", rep)
		}
	})
}

// newUUID returns a random version 4 UUID.
func newUUID(t *testing.T) string {
	t.Helper()
	h := randomHex(t, 16)
	return h[0:8] + "-" + h[8:12] + "-4" + h[13:16] + "-8" + h[17:20] + "-" + h[20:32]
}
