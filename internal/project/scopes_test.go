package project

import (
	"fmt"
	"testing"
	"wrk/internal/ticket"
)

func TestDeepDescendantScope(t *testing.T) {
	s := &Snapshot{ByID: map[string]*ticket.Ticket{}}
	for i := 0; i < 200; i++ {
		id := fmt.Sprintf("wrk-%08x", i)
		tkt := &ticket.Ticket{ID: id, Status: "todo", Labels: []string{"burn"}}
		if i > 0 {
			parent := fmt.Sprintf("wrk-%08x", i-1)
			tkt.Parent = &parent
		}
		s.Tickets = append(s.Tickets, tkt)
		s.ByID[id] = tkt
	}
	root := "wrk-00000000"
	if got := s.ScopedList(false, true, []string{"burn"}, &root); len(got) != 199 || got[0].ID != "wrk-00000001" || got[198].ID != "wrk-000000c7" {
		t.Fatalf("wrong deep scope: %d", len(got))
	}
	if s.Descendants(root)[root] {
		t.Fatal("included root")
	}
}
