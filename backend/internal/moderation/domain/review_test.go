package domain

import (
	"reflect"
	"testing"
)

func TestGroupReports(t *testing.T) {
	q1 := TargetRef{Kind: TargetQuote, ID: "q1"}
	c1 := TargetRef{Kind: TargetConversation, ID: "c1"}
	// A quote and a conversation can share an id; the kind tells them apart.
	sameID := TargetRef{Kind: TargetConversation, ID: "q1"}
	reports := []Report{
		{ID: "r1", Target: q1, OwnerID: "ada", OwnerUsername: "ada"},
		{ID: "r2", Target: c1, OwnerID: "bo", OwnerUsername: "bo"},
		{ID: "r3", Target: q1, OwnerID: "ada", OwnerUsername: "ada"},
		{ID: "r4", Target: sameID, OwnerID: "cy", OwnerUsername: "cy"},
	}
	got := GroupReports(reports)
	type group struct {
		ref     TargetRef
		owner   string
		reports []string
	}
	var summary []group
	for _, g := range got {
		ids := []string{}
		for _, r := range g.Reports {
			ids = append(ids, r.ID)
		}
		if g.Content != nil {
			t.Errorf("%v: content is attached later, got %#v", g.Ref, g.Content)
		}
		summary = append(summary, group{g.Ref, g.OwnerUsername, ids})
	}
	want := []group{
		{q1, "ada", []string{"r1", "r3"}},
		{c1, "bo", []string{"r2"}},
		{sameID, "cy", []string{"r4"}},
	}
	if !reflect.DeepEqual(summary, want) {
		t.Errorf("GroupReports() = %+v, want %+v", summary, want)
	}

	if got := GroupReports(nil); got == nil || len(got) != 0 {
		t.Errorf("GroupReports(nil) = %#v, want empty", got)
	}
}
