package workspace

import (
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/graph"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/testutil"
)

// newQueryTestWorkspace builds a minimal Workspace whose reloadable state
// holds only the given metamodel — no search index, no automation engine.
// Query tests don't exercise Search or automation, so we skip those to
// keep the test setup lean.
func newQueryTestWorkspace(g *graph.Graph, meta *metamodel.Metamodel) *Workspace {
	ws := &Workspace{graph: g}
	ws.state.Store(&workspaceState{meta: meta})
	return ws
}

func TestEntityQueries(t *testing.T) {
	meta := testutil.WorkspaceMetamodel()
	g := graph.New()
	ws := newQueryTestWorkspace(g, meta)

	// Add test entities
	g.AddNode(testutil.EntityFor(meta, "requirement").ID("REQ-001").With("title", "Test Requirement").Build())
	g.AddNode(testutil.EntityFor(meta, "decision").ID("DEC-001").With("title", "Test Decision").Build())

	// Test GetEntity
	entity, ok := ws.GetEntity("REQ-001")
	if !ok {
		t.Error("expected to find REQ-001")
	}
	if entity.Title() != "Test Requirement" {
		t.Errorf("got title %q, want %q", entity.Title(), "Test Requirement")
	}

	// Test GetEntity not found
	_, ok = ws.GetEntity("NONEXISTENT")
	if ok {
		t.Error("expected not to find NONEXISTENT")
	}

	// Test AllEntities
	entities := ws.AllEntities()
	if len(entities) != 2 {
		t.Errorf("got %d entities, want 2", len(entities))
	}

	// Test EntitiesByType
	reqs := ws.EntitiesByType("requirement")
	if len(reqs) != 1 {
		t.Errorf("got %d requirements, want 1", len(reqs))
	}

	// Test EntityCount
	if ws.EntityCount() != 2 {
		t.Errorf("got count %d, want 2", ws.EntityCount())
	}

	// Test EntityIDs
	ids := ws.EntityIDs()
	if len(ids) != 2 {
		t.Errorf("got %d IDs, want 2", len(ids))
	}
}

func TestRelationQueries(t *testing.T) {
	meta := testutil.WorkspaceMetamodel()
	g := graph.New()
	ws := newQueryTestWorkspace(g, meta)

	// Add entities
	g.AddNode(testutil.EntityFor(meta, "requirement").ID("REQ-001").Build())
	g.AddNode(testutil.EntityFor(meta, "decision").ID("DEC-001").Build())

	// Add relation
	g.AddEdge(testutil.NewRelation("DEC-001", "implements", "REQ-001").Build())

	// Test GetRelation
	found, ok := ws.GetRelation("DEC-001", "implements", "REQ-001")
	if !ok {
		t.Error("expected to find relation")
	}
	if found.From != "DEC-001" || found.To != "REQ-001" {
		t.Error("relation endpoints don't match")
	}

	// Test GetRelation not found
	_, ok = ws.GetRelation("REQ-001", "implements", "DEC-001")
	if ok {
		t.Error("expected not to find reverse relation")
	}

	// Test AllRelations
	relations := ws.AllRelations()
	if len(relations) != 1 {
		t.Errorf("got %d relations, want 1", len(relations))
	}

	// Test IncomingRelations
	incoming := ws.IncomingRelations("REQ-001")
	if len(incoming) != 1 {
		t.Errorf("got %d incoming, want 1", len(incoming))
	}

	// Test OutgoingRelations
	outgoing := ws.OutgoingRelations("DEC-001")
	if len(outgoing) != 1 {
		t.Errorf("got %d outgoing, want 1", len(outgoing))
	}
}

func TestGraphAnalysis(t *testing.T) {
	meta := testutil.WorkspaceMetamodel()
	g := graph.New()
	ws := newQueryTestWorkspace(g, meta)

	// Add entities
	g.AddNode(testutil.EntityFor(meta, "requirement").ID("REQ-001").Build())
	g.AddNode(testutil.EntityFor(meta, "decision").ID("DEC-001").Build())
	g.AddNode(testutil.EntityFor(meta, "requirement").ID("ORPHAN-001").Build())

	// Add relation between req and dec
	g.AddEdge(testutil.NewRelation("DEC-001", "implements", "REQ-001").Build())

	// Test FindOrphans
	orphans := ws.FindOrphans()
	if len(orphans) != 1 {
		t.Errorf("got %d orphans, want 1", len(orphans))
	}
	if orphans[0].ID != "ORPHAN-001" {
		t.Errorf("got orphan %q, want ORPHAN-001", orphans[0].ID)
	}

	// Test TraceFrom
	trace := ws.TraceFrom("DEC-001", 0)
	if trace == nil {
		t.Error("expected trace result, got nil")
	}

	// Test TraceTo
	trace = ws.TraceTo("REQ-001", 0)
	if trace == nil {
		t.Error("expected trace result, got nil")
	}

	// Test FindPath
	path := ws.FindPath("DEC-001", "REQ-001")
	if len(path) == 0 {
		t.Error("expected path, got empty")
	}
}
