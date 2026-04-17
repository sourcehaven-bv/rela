package dataentry

import (
	"context"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
)

func newTestStoreGraph(t *testing.T) *StoreGraph {
	t.Helper()
	s := memstore.New()

	ctx := context.Background()
	e1 := entity.New("TKT-001", "ticket")
	e1.SetString("title", "First Ticket")
	e1.SetString("status", "open")
	if err := s.CreateEntity(ctx, e1); err != nil {
		t.Fatal(err)
	}

	e2 := entity.New("TKT-002", "ticket")
	e2.SetString("title", "Second Ticket")
	if err := s.CreateEntity(ctx, e2); err != nil {
		t.Fatal(err)
	}

	e3 := entity.New("CMP-001", "component")
	e3.SetString("title", "Component")
	if err := s.CreateEntity(ctx, e3); err != nil {
		t.Fatal(err)
	}

	if _, err := s.CreateRelation(ctx, "TKT-001", "depends_on", "TKT-002", nil); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateRelation(ctx, "TKT-001", "belongs_to", "CMP-001", nil); err != nil {
		t.Fatal(err)
	}

	return NewStoreGraph(s)
}

// Compile-time check that StoreGraph satisfies EntityGraph.
var _ EntityGraph = (*StoreGraph)(nil)

func TestStoreGraph_GetNode(t *testing.T) {
	sg := newTestStoreGraph(t)

	e, ok := sg.GetNode("TKT-001")
	if !ok {
		t.Fatal("expected to find TKT-001")
	}
	if e.ID != "TKT-001" || e.Type != "ticket" {
		t.Errorf("got %s/%s", e.ID, e.Type)
	}
	if e.GetString("title") != "First Ticket" {
		t.Errorf("title: got %q", e.GetString("title"))
	}

	_, ok = sg.GetNode("NOPE")
	if ok {
		t.Error("expected not found")
	}
}

func TestStoreGraph_NodesByType(t *testing.T) {
	sg := newTestStoreGraph(t)

	tickets := sg.NodesByType("ticket")
	if len(tickets) != 2 {
		t.Fatalf("expected 2 tickets, got %d", len(tickets))
	}

	components := sg.NodesByType("component")
	if len(components) != 1 {
		t.Fatalf("expected 1 component, got %d", len(components))
	}

	none := sg.NodesByType("nonexistent")
	if len(none) != 0 {
		t.Fatalf("expected 0, got %d", len(none))
	}
}

func TestStoreGraph_AllNodes(t *testing.T) {
	sg := newTestStoreGraph(t)
	all := sg.AllNodes()
	if len(all) != 3 {
		t.Fatalf("expected 3, got %d", len(all))
	}
}

func TestStoreGraph_AllIDs(t *testing.T) {
	sg := newTestStoreGraph(t)
	ids := sg.AllIDs()
	if len(ids) != 3 {
		t.Fatalf("expected 3, got %d", len(ids))
	}
}

func TestStoreGraph_AllEdges(t *testing.T) {
	sg := newTestStoreGraph(t)
	edges := sg.AllEdges()
	if len(edges) != 2 {
		t.Fatalf("expected 2, got %d", len(edges))
	}
}

func TestStoreGraph_OutgoingEdges(t *testing.T) {
	sg := newTestStoreGraph(t)

	out := sg.OutgoingEdges("TKT-001")
	if len(out) != 2 {
		t.Fatalf("expected 2 outgoing from TKT-001, got %d", len(out))
	}

	out = sg.OutgoingEdges("TKT-002")
	if len(out) != 0 {
		t.Fatalf("expected 0 outgoing from TKT-002, got %d", len(out))
	}
}

func TestStoreGraph_IncomingEdges(t *testing.T) {
	sg := newTestStoreGraph(t)

	in := sg.IncomingEdges("TKT-002")
	if len(in) != 1 {
		t.Fatalf("expected 1 incoming to TKT-002, got %d", len(in))
	}
	if in[0].From != "TKT-001" {
		t.Errorf("expected from TKT-001, got %s", in[0].From)
	}
}

func TestStoreGraph_FindOrphans(t *testing.T) {
	sg := newTestStoreGraph(t)

	// TKT-002 and CMP-001 have relations, TKT-001 has relations → no orphans
	orphans := sg.FindOrphans()
	if len(orphans) != 0 {
		t.Fatalf("expected 0 orphans, got %d", len(orphans))
	}

	// Add an orphan entity
	ctx := context.Background()
	orphan := entity.New("ORPHAN-001", "ticket")
	if err := sg.store.CreateEntity(ctx, orphan); err != nil {
		t.Fatal(err)
	}

	orphans = sg.FindOrphans()
	if len(orphans) != 1 {
		t.Fatalf("expected 1 orphan, got %d", len(orphans))
	}
	if orphans[0].ID != "ORPHAN-001" {
		t.Errorf("expected ORPHAN-001, got %s", orphans[0].ID)
	}
}
