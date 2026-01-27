package graph

import (
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/model"
)

// Test helpers to avoid import cycle
func assertEqual(t *testing.T, got, want interface{}) {
	t.Helper()
	if got != want {
		t.Errorf("got %v, want %v", got, want)
	}
}

func newEntity(id, entityType string) *model.Entity {
	return model.NewEntity(id, entityType)
}

func newRelation(from, relationType, to string) *model.Relation {
	return model.NewRelation(from, relationType, to)
}

func TestNew(t *testing.T) {
	g := New()
	if g == nil {
		t.Fatal("expected non-nil graph")
	}
	if g.NodeCount() != 0 {
		t.Error("expected new graph to be empty")
	}
}

func TestAddNode(t *testing.T) {
	g := New()
	e := newEntity("TEST-001", "test")

	g.AddNode(e)

	assertEqual(t, g.NodeCount(), 1)

	retrieved, ok := g.GetNode("TEST-001")
	if !ok {
		t.Error("expected to find added node")
	}
	assertEqual(t, retrieved.ID, "TEST-001")
}

func TestUpdateNode(t *testing.T) {
	g := New()
	e := newEntity("TEST-001", "test")
	e.Properties["title"] = "Original"

	g.AddNode(e)

	// Update existing node
	e2 := newEntity("TEST-001", "test")
	e2.Properties["title"] = "Updated"
	ok := g.UpdateNode(e2)
	if !ok {
		t.Error("expected UpdateNode to succeed for existing node")
	}

	retrieved, _ := g.GetNode("TEST-001")
	assertEqual(t, retrieved.Properties["title"], "Updated")

	// Try to update non-existent node
	e3 := newEntity("NONEXISTENT", "test")
	ok = g.UpdateNode(e3)
	if ok {
		t.Error("expected UpdateNode to fail for non-existent node")
	}
}

func TestGetNode(t *testing.T) {
	g := New()
	e := newEntity("TEST-001", "test")
	g.AddNode(e)

	// Get existing node
	retrieved, ok := g.GetNode("TEST-001")
	if !ok {
		t.Error("expected to find node")
	}
	assertEqual(t, retrieved.ID, "TEST-001")

	// Get non-existent node
	_, ok = g.GetNode("NONEXISTENT")
	if ok {
		t.Error("expected not to find non-existent node")
	}
}

func TestRemoveNode(t *testing.T) {
	g := New()
	e1 := newEntity("TEST-001", "test")
	e2 := newEntity("TEST-002", "test")
	g.AddNode(e1)
	g.AddNode(e2)

	r := newRelation("TEST-001", "links_to", "TEST-002")
	g.AddEdge(r)

	// Remove node
	ok := g.RemoveNode("TEST-001")
	if !ok {
		t.Error("expected RemoveNode to succeed")
	}

	assertEqual(t, g.NodeCount(), 1)

	// Relations should be removed too
	assertEqual(t, g.EdgeCount(), 0)

	// Try to remove non-existent node
	ok = g.RemoveNode("NONEXISTENT")
	if ok {
		t.Error("expected RemoveNode to fail for non-existent node")
	}
}

func TestAddEdge(t *testing.T) {
	g := New()
	r := newRelation("TEST-001", "links_to", "TEST-002")

	g.AddEdge(r)

	assertEqual(t, g.EdgeCount(), 1)

	edges := g.OutgoingEdges("TEST-001")
	assertEqual(t, len(edges), 1)

	edges = g.IncomingEdges("TEST-002")
	assertEqual(t, len(edges), 1)
}

func TestRemoveEdge(t *testing.T) {
	g := New()
	r := newRelation("TEST-001", "links_to", "TEST-002")
	g.AddEdge(r)

	// Remove existing edge
	ok := g.RemoveEdge("TEST-001", "links_to", "TEST-002")
	if !ok {
		t.Error("expected RemoveEdge to succeed")
	}

	assertEqual(t, g.EdgeCount(), 0)

	// Try to remove non-existent edge
	ok = g.RemoveEdge("NONEXISTENT", "links_to", "TEST-002")
	if ok {
		t.Error("expected RemoveEdge to fail for non-existent edge")
	}
}

func TestGetEdge(t *testing.T) {
	g := New()
	r := newRelation("TEST-001", "links_to", "TEST-002")
	g.AddEdge(r)

	// Get existing edge
	edge, ok := g.GetEdge("TEST-001", "links_to", "TEST-002")
	if !ok {
		t.Error("expected to find edge")
	}
	assertEqual(t, edge.From, "TEST-001")

	// Get non-existent edge
	_, ok = g.GetEdge("NONEXISTENT", "links_to", "TEST-002")
	if ok {
		t.Error("expected not to find non-existent edge")
	}
}

func TestOutgoingEdges(t *testing.T) {
	g := New()
	r1 := newRelation("TEST-001", "links_to", "TEST-002")
	r2 := newRelation("TEST-001", "links_to", "TEST-003")
	g.AddEdge(r1)
	g.AddEdge(r2)

	edges := g.OutgoingEdges("TEST-001")
	assertEqual(t, len(edges), 2)

	// Non-existent node
	edges = g.OutgoingEdges("NONEXISTENT")
	assertEqual(t, len(edges), 0)
}

func TestIncomingEdges(t *testing.T) {
	g := New()
	r1 := newRelation("TEST-001", "links_to", "TEST-003")
	r2 := newRelation("TEST-002", "links_to", "TEST-003")
	g.AddEdge(r1)
	g.AddEdge(r2)

	edges := g.IncomingEdges("TEST-003")
	assertEqual(t, len(edges), 2)

	// Non-existent node
	edges = g.IncomingEdges("NONEXISTENT")
	assertEqual(t, len(edges), 0)
}

func TestAllNodes(t *testing.T) {
	g := New()
	e1 := newEntity("TEST-001", "test")
	e2 := newEntity("TEST-002", "test")
	g.AddNode(e1)
	g.AddNode(e2)

	nodes := g.AllNodes()
	assertEqual(t, len(nodes), 2)
}

func TestAllEdges(t *testing.T) {
	g := New()
	r1 := newRelation("TEST-001", "links_to", "TEST-002")
	r2 := newRelation("TEST-002", "links_to", "TEST-003")
	g.AddEdge(r1)
	g.AddEdge(r2)

	edges := g.AllEdges()
	assertEqual(t, len(edges), 2)
}

func TestNodesByType(t *testing.T) {
	g := New()
	e1 := newEntity("REQ-001", "requirement")
	e2 := newEntity("REQ-002", "requirement")
	e3 := newEntity("DEC-001", "decision")
	g.AddNode(e1)
	g.AddNode(e2)
	g.AddNode(e3)

	reqs := g.NodesByType("requirement")
	assertEqual(t, len(reqs), 2)

	decs := g.NodesByType("decision")
	assertEqual(t, len(decs), 1)

	// Non-existent type
	none := g.NodesByType("nonexistent")
	assertEqual(t, len(none), 0)
}

func TestNodeCount(t *testing.T) {
	g := New()
	assertEqual(t, g.NodeCount(), 0)

	g.AddNode(newEntity("TEST-001", "test"))
	assertEqual(t, g.NodeCount(), 1)
}

func TestEdgeCount(t *testing.T) {
	g := New()
	assertEqual(t, g.EdgeCount(), 0)

	g.AddEdge(newRelation("TEST-001", "links_to", "TEST-002"))
	assertEqual(t, g.EdgeCount(), 1)
}

func TestAllIDs(t *testing.T) {
	g := New()
	g.AddNode(newEntity("TEST-001", "test"))
	g.AddNode(newEntity("TEST-002", "test"))

	ids := g.AllIDs()
	assertEqual(t, len(ids), 2)

	found1, found2 := false, false
	for _, id := range ids {
		if id == "TEST-001" {
			found1 = true
		}
		if id == "TEST-002" {
			found2 = true
		}
	}
	if !found1 || !found2 {
		t.Error("expected to find both IDs")
	}
}

func TestIDsByType(t *testing.T) {
	g := New()
	g.AddNode(newEntity("REQ-001", "requirement"))
	g.AddNode(newEntity("REQ-002", "requirement"))
	g.AddNode(newEntity("DEC-001", "decision"))

	reqIDs := g.IDsByType("requirement")
	assertEqual(t, len(reqIDs), 2)

	decIDs := g.IDsByType("decision")
	assertEqual(t, len(decIDs), 1)
}

func TestClear(t *testing.T) {
	g := New()
	g.AddNode(newEntity("TEST-001", "test"))
	g.AddEdge(newRelation("TEST-001", "links_to", "TEST-002"))

	g.Clear()

	assertEqual(t, g.NodeCount(), 0)
	assertEqual(t, g.EdgeCount(), 0)
}

func TestTraceTo(t *testing.T) {
	g := New()
	a := newEntity("A", "test")
	a.Properties["title"] = "A"
	b := newEntity("B", "test")
	b.Properties["title"] = "B"
	c := newEntity("C", "test")
	c.Properties["title"] = "C"

	g.AddNode(a)
	g.AddNode(b)
	g.AddNode(c)

	g.AddEdge(newRelation("A", "links_to", "B"))
	g.AddEdge(newRelation("B", "links_to", "C"))

	result := g.TraceTo("C", 10)
	if result == nil {
		t.Fatal("expected trace result")
	}
	assertEqual(t, result.ID, "C")
	if len(result.Children) == 0 {
		t.Error("expected trace to have children")
	}
}

func TestFindOrphans(t *testing.T) {
	g := New()
	g.AddNode(newEntity("ORPHAN", "test"))
	g.AddNode(newEntity("CONNECTED", "test"))
	g.AddEdge(newRelation("CONNECTED", "links_to", "OTHER"))

	orphans := g.FindOrphans()
	if len(orphans) == 0 {
		t.Error("expected to find orphan nodes")
	}

	found := false
	for _, o := range orphans {
		if o.ID == "ORPHAN" {
			found = true
		}
	}
	if !found {
		t.Error("expected to find ORPHAN node")
	}
}

func TestFindClusters(t *testing.T) {
	g := New()
	g.AddNode(newEntity("A", "test"))
	g.AddNode(newEntity("B", "test"))
	g.AddNode(newEntity("C", "test"))
	g.AddNode(newEntity("D", "test"))

	// Cluster 1: A-B
	g.AddEdge(newRelation("A", "links_to", "B"))

	// Cluster 2: C-D
	g.AddEdge(newRelation("C", "links_to", "D"))

	clusters := g.FindClusters()
	assertEqual(t, len(clusters), 2)
}

func TestHasCycle(t *testing.T) {
	g := New()
	g.AddNode(newEntity("A", "test"))
	g.AddNode(newEntity("B", "test"))
	g.AddNode(newEntity("C", "test"))

	// Create cycle: A -> B -> C -> A
	g.AddEdge(newRelation("A", "links_to", "B"))
	g.AddEdge(newRelation("B", "links_to", "C"))
	g.AddEdge(newRelation("C", "links_to", "A"))

	hasCycle := g.HasCycle("A")
	if !hasCycle {
		t.Error("expected to detect cycle")
	}

	// No cycle graph
	g2 := New()
	g2.AddNode(newEntity("X", "test"))
	g2.AddNode(newEntity("Y", "test"))
	g2.AddEdge(newRelation("X", "links_to", "Y"))

	hasCycle2 := g2.HasCycle("X")
	if hasCycle2 {
		t.Error("expected no cycle in linear graph")
	}
}

func TestRelationsOfType(t *testing.T) {
	g := New()
	g.AddEdge(newRelation("A", "implements", "B"))
	g.AddEdge(newRelation("C", "depends_on", "D"))
	g.AddEdge(newRelation("E", "implements", "F"))

	implements := g.RelationsOfType("implements")
	assertEqual(t, len(implements), 2)

	dependsOn := g.RelationsOfType("depends_on")
	assertEqual(t, len(dependsOn), 1)
}
