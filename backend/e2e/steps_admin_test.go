//go:build e2e

package e2e

import (
	"net/http"
	"testing"
)

// adminRoute is one /admin route with a valid body.
type adminRoute struct {
	method string
	path   string
	body   any
}

func (s *scenario) adminRoutes(t *testing.T) []adminRoute {
	t.Helper()
	target := map[string]string{"target_kind": "quote", "target_id": newUUID(t)}
	return []adminRoute{
		{http.MethodGet, "/admin/reports", nil},
		{http.MethodPost, "/admin/reports/dismiss", target},
		{http.MethodGet, "/admin/takedowns", nil},
		{http.MethodPost, "/admin/takedowns", map[string]string{
			"target_kind": "quote", "target_id": newUUID(t), "reason": "e2e"}},
		{http.MethodPost, "/admin/takedowns/" + newUUID(t) + "/restore", map[string]string{}},
	}
}

// stepAdmin: only admins reach /admin; a hidden quote leaves every public
// read and can't be republished or reported; restoring brings it back.
func (s *scenario) stepAdmin(t *testing.T) {
	t.Run("every admin route is 403 for a reader", func(t *testing.T) {
		for _, rt := range s.adminRoutes(t) {
			r := s.api.call(t, rt.method, rt.path, s.b.token, rt.body)
			if r.status != http.StatusForbidden {
				t.Errorf("want HTTP 403 %q, got %s", adminsOnlyMsg, r)
			}
		}
	})
	t.Run("every admin route is 401 signed out", func(t *testing.T) {
		for _, rt := range s.adminRoutes(t) {
			if r := s.api.call(t, rt.method, rt.path, "", rt.body); r.status != http.StatusUnauthorized {
				t.Errorf("want HTTP 401, got %s", r)
			}
		}
	})
	t.Run("C is made an admin with SQL", func(t *testing.T) {
		if err := makeAdmin(t.Context(), s.db, s.c.id); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("the admin without reader terms lists the 2 reported targets", func(t *testing.T) {
		need(t, s.q1, s.conv)
		me := expectDecode[meBody](t, s.api.call(t, http.MethodGet, "/me", s.c.token, nil), http.StatusOK)
		if me.TermsAccepted {
			t.Fatal("the admin has accepted the reader terms; the check needs an admin who hasn't")
		}
		got := s.reportedTargets(t)
		if len(got) != 2 || len(got[s.q1].Reports) != 1 || len(got[s.conv].Reports) != 1 {
			t.Fatalf("reported targets of A: %+v, want q1 and the discussion with 1 report each", got)
		}
		if got[s.q1].OwnerUsername != s.a.username || got[s.q1].Reports[0].ReporterID != s.b.id {
			t.Errorf("q1 target %+v, want owner %s reported by B", got[s.q1], s.a.username)
		}
	})
	t.Run("hiding the quote is 201", func(t *testing.T) {
		need(t, s.q1)
		td := expectDecode[takedownBody](t, s.api.call(t, http.MethodPost, "/admin/takedowns", s.c.token,
			map[string]string{"target_kind": "quote", "target_id": s.q1, "reason": takedownReason}), http.StatusCreated)
		s.takedown = td.ID
		if td.TargetID != s.q1 || td.HiddenBy == nil || *td.HiddenBy != s.c.id || td.RestoredAt != nil {
			t.Fatalf("takedown %+v, want an active one on q1 by C", td)
		}
	})
	t.Run("hiding it again is 409", func(t *testing.T) {
		need(t, s.q1)
		r := s.api.call(t, http.MethodPost, "/admin/takedowns", s.c.token,
			map[string]string{"target_kind": "quote", "target_id": s.q1, "reason": takedownReason})
		expectError(t, r, http.StatusConflict, "this is already hidden")
	})
	s.checkHidden(t)
	s.checkRestore(t)
}

// checkHidden: what the hidden quote looks like to everyone.
func (s *scenario) checkHidden(t *testing.T) {
	a := s.a
	t.Run("the quote leaves the profile with its discussion count and trail point", func(t *testing.T) {
		need(t, s.takedown)
		p := s.publicProfile(t)
		if p.quote(s.q1) != nil || p.hasConversation(s.conv) {
			t.Error("q1 or its discussion is still on the profile")
		}
		book := p.book(s.workA)
		if book == nil || book.PublicQuotes != 1 || len(book.Trail.Points) != 1 || book.Trail.Points[0].QuoteID != s.q2 {
			t.Errorf("shared book %+v, want 1 public quote and only q2 on the trail", book)
		}
	})
	t.Run("the owner sees the removed reason", func(t *testing.T) {
		need(t, s.takedown)
		q := expectDecode[quoteBody](t, s.api.call(t, http.MethodGet, "/quotes/"+s.q1, a.token, nil), http.StatusOK)
		if q.RemovedAt == nil || q.RemovedReason != takedownReason {
			t.Fatalf("removed_at %v, removed_reason %q", q.RemovedAt, q.RemovedReason)
		}
	})
	t.Run("publishing the removed quote is 409", func(t *testing.T) {
		need(t, s.takedown)
		r := s.api.call(t, http.MethodPatch, "/quotes/"+s.q1, a.token, map[string]string{"visibility": "public"})
		expectError(t, r, http.StatusConflict, "a moderator removed this quote")
	})
	t.Run("reporting the removed quote is 404", func(t *testing.T) {
		need(t, s.takedown)
		r := s.api.call(t, http.MethodPost, "/reports", s.b.token, reportReq("quote", s.q1))
		expectError(t, r, http.StatusNotFound, "not found")
	})
	t.Run("the quote's report is closed and the discussion's is open", func(t *testing.T) {
		need(t, s.takedown)
		got := s.reportedTargets(t)
		if _, open := got[s.q1]; open || len(got) != 1 || len(got[s.conv].Reports) != 1 {
			t.Fatalf("reported targets of A: %+v, want only the discussion", got)
		}
	})
	t.Run("dismissing the discussion's report closes 1", func(t *testing.T) {
		need(t, s.conv)
		got := expectDecode[dismissedBody](t, s.api.call(t, http.MethodPost, "/admin/reports/dismiss", s.c.token,
			map[string]string{"target_kind": "conversation", "target_id": s.conv}), http.StatusOK)
		if got.Dismissed != 1 {
			t.Fatalf("dismissed %d, want 1", got.Dismissed)
		}
	})
	t.Run("dismissing it again is 404", func(t *testing.T) {
		need(t, s.conv)
		r := s.api.call(t, http.MethodPost, "/admin/reports/dismiss", s.c.token,
			map[string]string{"target_kind": "conversation", "target_id": s.conv})
		expectError(t, r, http.StatusNotFound, "no open reports on this")
	})
}

// checkRestore: the takedown is listed, restored once, and the quote is
// back where it was.
func (s *scenario) checkRestore(t *testing.T) {
	a := s.a
	t.Run("one active takedown is listed", func(t *testing.T) {
		need(t, s.takedown)
		all := expectDecode[[]takedownBody](t,
			s.api.call(t, http.MethodGet, "/admin/takedowns", s.c.token, nil), http.StatusOK)
		var ours []takedownBody
		for _, td := range all {
			if td.OwnerID == a.id {
				ours = append(ours, td)
			}
		}
		if len(ours) != 1 || ours[0].ID != s.takedown {
			t.Fatalf("active takedowns of A: %+v, want only %s", ours, s.takedown)
		}
	})
	t.Run("restoring is 200", func(t *testing.T) {
		need(t, s.takedown)
		td := expectDecode[takedownBody](t, s.api.call(t, http.MethodPost,
			"/admin/takedowns/"+s.takedown+"/restore", s.c.token, map[string]string{}), http.StatusOK)
		if td.RestoredAt == nil || td.RestoredBy == nil || *td.RestoredBy != s.c.id {
			t.Fatalf("takedown %+v, want restored by C", td)
		}
	})
	t.Run("restoring again is 409", func(t *testing.T) {
		need(t, s.takedown)
		r := s.api.call(t, http.MethodPost, "/admin/takedowns/"+s.takedown+"/restore", s.c.token, map[string]string{})
		expectError(t, r, http.StatusConflict, "this was already restored")
	})
	t.Run("the quote and its discussion are back", func(t *testing.T) {
		need(t, s.q1, s.conv)
		p := s.publicProfile(t)
		q1 := p.quote(s.q1)
		if q1 == nil || !p.hasConversation(s.conv) || q1.RemovedAt != nil {
			t.Fatalf("q1 %+v, want it listed with the discussion", q1)
		}
		if book := p.book(s.workA); book == nil || book.PublicQuotes != 2 || len(book.Trail.Points) != 2 {
			t.Errorf("shared book %+v, want 2 public quotes on the trail", book)
		}
	})
	t.Run("the owner can unpublish and republish", func(t *testing.T) {
		need(t, s.q1)
		q := expectDecode[quoteBody](t, s.api.call(t, http.MethodPatch, "/quotes/"+s.q1, a.token,
			map[string]string{"visibility": "private"}), http.StatusOK)
		if q.Visibility != "private" {
			t.Fatalf("visibility %s after unpublishing", q.Visibility)
		}
		if q = s.publish(t, a, s.q1); q.RemovedAt != nil {
			t.Fatalf("republished quote still has removed_at %s", *q.RemovedAt)
		}
	})
}

// reportedTargets returns the open reported targets owned by A, by target id.
func (s *scenario) reportedTargets(t *testing.T) map[string]reportedTargetBody {
	t.Helper()
	all := expectDecode[[]reportedTargetBody](t,
		s.api.call(t, http.MethodGet, "/admin/reports", s.c.token, nil), http.StatusOK)
	ours := map[string]reportedTargetBody{}
	for _, rt := range all {
		if rt.OwnerID == s.a.id {
			ours[rt.TargetID] = rt
		}
	}
	return ours
}
