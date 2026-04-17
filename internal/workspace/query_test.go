package workspace

import (
	"context"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
)

func queryTestWorkspace(t *testing.T) *Workspace {
	t.Helper()
	s := memstore.New()
	ctx := context.Background()

	for _, e := range []*entity.Entity{
		entity.New("A-1", "ticket"),
		entity.New("A-2", "ticket"),
		entity.New("C-1", "component"),
	} {
		if err := s.CreateEntity(ctx, e); err != nil {
			t.Fatalf("seed entity: %v", err)
		}
	}
	if _, err := s.CreateRelation(ctx, "A-1", "depends", "A-2", nil); err != nil {
		t.Fatalf("seed relation: %v", err)
	}
	if _, err := s.CreateRelation(ctx, "A-2", "owns", "C-1", nil); err != nil {
		t.Fatalf("seed relation: %v", err)
	}

	meta := &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"ticket":    {Label: "Ticket"},
			"component": {Label: "Component"},
		},
		Relations: map[string]metamodel.RelationDef{
			"depends": {From: []string{"ticket"}, To: []string{"ticket"}},
			"owns":    {From: []string{"ticket"}, To: []string{"component"}},
		},
	}
	return NewForTestWithStore(s, meta)
}

func TestGetEntity(t *testing.T) {
	ws := queryTestWorkspace(t)

	e, ok := ws.GetEntity("A-1")
	if !ok {
		t.Fatal("expected A-1 to exist")
	}
	if e.Type != "ticket" {
		t.Errorf("type = %q, want ticket", e.Type)
	}

	if _, ok := ws.GetEntity("MISSING"); ok {
		t.Error("expected missing ID to return ok=false")
	}
}

func TestGetRelation(t *testing.T) {
	ws := queryTestWorkspace(t)

	r, ok := ws.GetRelation("A-1", "depends", "A-2")
	if !ok {
		t.Fatal("expected relation to exist")
	}
	if r.From != "A-1" || r.To != "A-2" || r.Type != "depends" {
		t.Errorf("unexpected relation: %+v", r)
	}

	if _, ok := ws.GetRelation("A-1", "nope", "A-2"); ok {
		t.Error("expected missing relation to return ok=false")
	}
}

func TestIncomingAndOutgoingRelations(t *testing.T) {
	ws := queryTestWorkspace(t)

	out := ws.OutgoingRelations("A-1")
	if len(out) != 1 || out[0].To != "A-2" {
		t.Errorf("A-1 outgoing = %+v", out)
	}

	in := ws.IncomingRelations("A-2")
	if len(in) != 1 || in[0].From != "A-1" {
		t.Errorf("A-2 incoming = %+v", in)
	}

	// Component with one incoming, no outgoing.
	if got := ws.OutgoingRelations("C-1"); len(got) != 0 {
		t.Errorf("C-1 should have no outgoing, got %+v", got)
	}
	if got := ws.IncomingRelations("C-1"); len(got) != 1 {
		t.Errorf("C-1 should have 1 incoming, got %+v", got)
	}

	// Unknown entity returns empty, not nil-panicking.
	if got := ws.OutgoingRelations("MISSING"); len(got) != 0 {
		t.Errorf("missing entity outgoing = %+v", got)
	}
}
