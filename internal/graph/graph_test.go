package graph

import (
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/model"
)

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
	e := &model.Entity{ID: "TEST-001", Type: "test"}

	g.AddNode(e)

	if g.NodeCount() != 1 {
		t.Errorf("expected 1 node, got %d", g.NodeCount())
	}

	retrieved, ok := g.GetNode("TEST-001")
	if !ok {
		t.Error("expected to find added node")
	}
	if retrieved.ID != "TEST-001" {
		t.Errorf("expected ID TEST-001, got %s", retrieved.ID)
	}
}

func TestUpdateNode(t *testing.T) {
	g := New()
	e := &model.Entity{ID: "TEST-001", Type: "test", Properties: map[string]interface{}{"title": "Original"}}

	g.AddNode(e)

	// Update existing node
	e2 := &model.Entity{ID: "TEST-001", Type: "test", Properties: map[string]interface{}{"title": "Updated"}}
	ok := g.UpdateNode(e2)
	if !ok {
		t.Error("expected UpdateNode to succeed for existing node")
	}

	retrieved, _ := g.GetNode("TEST-001")
	if retrieved.Properties["title"] != "Updated" {
		t.Error("expected node properties to be updated")
	}

	// Try to update non-existent node
	e3 := &model.Entity{ID: "NONEXISTENT", Type: "test"}
	ok = g.UpdateNode(e3)
	if ok {
		t.Error("expected UpdateNode to fail for non-existent node")
	}
}

func TestGetNode(t *testing.T) {
	g := New()
	e := &model.Entity{ID: "TEST-001", Type: "test"}
	g.AddNode(e)

	// Get existing node
	retrieved, ok := g.GetNode("TEST-001")
	if !ok {
		t.Error("expected to find node")
	}
	if retrieved.ID != "TEST-001" {
		t.Error("expected correct node ID")
	}

	// Get non-existent node
	_, ok = g.GetNode("NONEXISTENT")
	if ok {
		t.Error("expected not to find non-existent node")
	}
}

func TestRemoveNode(t *testing.T) {
	g := New()
	e1 := &model.Entity{ID: "TEST-001", Type: "test"}
	e2 := &model.Entity{ID: "TEST-002", Type: "test"}
	g.AddNode(e1)
	g.AddNode(e2)

	r := &model.Relation{From: "TEST-001", Type: "links_to", To: "TEST-002"}
	g.AddEdge(r)

	// Remove node
	ok := g.RemoveNode("TEST-001")
	if !ok {
		t.Error("expected RemoveNode to succeed")
	}

	if g.NodeCount() != 1 {
		t.Errorf("expected 1 node after removal, got %d", g.NodeCount())
	}

	// Relations should be removed too
	if g.EdgeCount() != 0 {
		t.Errorf("expected 0 edges after node removal, got %d", g.EdgeCount())
	}

	// Try to remove non-existent node
	ok = g.RemoveNode("NONEXISTENT")
	if ok {
		t.Error("expected RemoveNode to fail for non-existent node")
	}
}

func TestAddEdge(t *testing.T) {
	g := New()
	r := &model.Relation{From: "TEST-001", Type: "links_to", To: "TEST-002"}

	g.AddEdge(r)

	if g.EdgeCount() != 1 {
		t.Errorf("expected 1 edge, got %d", g.EdgeCount())
	}

	edges := g.OutgoingEdges("TEST-001")
	if len(edges) != 1 {
		t.Errorf("expected 1 outgoing edge, got %d", len(edges))
	}

	edges = g.IncomingEdges("TEST-002")
	if len(edges) != 1 {
		t.Errorf("expected 1 incoming edge, got %d", len(edges))
	}
}

func TestRemoveEdge(t *testing.T) {
	g := New()
	r := &model.Relation{From: "TEST-001", Type: "links_to", To: "TEST-002"}
	g.AddEdge(r)

	// Remove existing edge
	ok := g.RemoveEdge("TEST-001", "links_to", "TEST-002")
	if !ok {
		t.Error("expected RemoveEdge to succeed")
	}

	if g.EdgeCount() != 0 {
		t.Errorf("expected 0 edges after removal, got %d", g.EdgeCount())
	}

	// Try to remove non-existent edge
	ok = g.RemoveEdge("NONEXISTENT", "links_to", "TEST-002")
	if ok {
		t.Error("expected RemoveEdge to fail for non-existent edge")
	}
}

func TestGetEdge(t *testing.T) {
	g := New()
	r := &model.Relation{From: "TEST-001", Type: "links_to", To: "TEST-002"}
	g.AddEdge(r)

	// Get existing edge
	edge, ok := g.GetEdge("TEST-001", "links_to", "TEST-002")
	if !ok {
		t.Error("expected to find edge")
	}
	if edge.From != "TEST-001" {
		t.Error("expected correct edge")
	}

	// Get non-existent edge
	_, ok = g.GetEdge("NONEXISTENT", "links_to", "TEST-002")
	if ok {
		t.Error("expected not to find non-existent edge")
	}
}

func TestOutgoingEdges(t *testing.T) {
	g := New()
	r1 := &model.Relation{From: "TEST-001", Type: "links_to", To: "TEST-002"}
	r2 := &model.Relation{From: "TEST-001", Type: "links_to", To: "TEST-003"}
	g.AddEdge(r1)
	g.AddEdge(r2)

	edges := g.OutgoingEdges("TEST-001")
	if len(edges) != 2 {
		t.Errorf("expected 2 outgoing edges, got %d", len(edges))
	}

	// Non-existent node
	edges = g.OutgoingEdges("NONEXISTENT")
	if len(edges) != 0 {
		t.Errorf("expected 0 outgoing edges for non-existent node, got %d", len(edges))
	}
}

func TestIncomingEdges(t *testing.T) {
	g := New()
	r1 := &model.Relation{From: "TEST-001", Type: "links_to", To: "TEST-003"}
	r2 := &model.Relation{From: "TEST-002", Type: "links_to", To: "TEST-003"}
	g.AddEdge(r1)
	g.AddEdge(r2)

	edges := g.IncomingEdges("TEST-003")
	if len(edges) != 2 {
		t.Errorf("expected 2 incoming edges, got %d", len(edges))
	}

	// Non-existent node
	edges = g.IncomingEdges("NONEXISTENT")
	if len(edges) != 0 {
		t.Errorf("expected 0 incoming edges for non-existent node, got %d", len(edges))
	}
}

func TestAllNodes(t *testing.T) {
	g := New()
	e1 := &model.Entity{ID: "TEST-001", Type: "test"}
	e2 := &model.Entity{ID: "TEST-002", Type: "test"}
	g.AddNode(e1)
	g.AddNode(e2)

	nodes := g.AllNodes()
	if len(nodes) != 2 {
		t.Errorf("expected 2 nodes, got %d", len(nodes))
	}
}

func TestAllEdges(t *testing.T) {
	g := New()
	r1 := &model.Relation{From: "TEST-001", Type: "links_to", To: "TEST-002"}
	r2 := &model.Relation{From: "TEST-002", Type: "links_to", To: "TEST-003"}
	g.AddEdge(r1)
	g.AddEdge(r2)

	edges := g.AllEdges()
	if len(edges) != 2 {
		t.Errorf("expected 2 edges, got %d", len(edges))
	}
}

func TestNodesByType(t *testing.T) {
	g := New()
	e1 := &model.Entity{ID: "REQ-001", Type: "requirement"}
	e2 := &model.Entity{ID: "REQ-002", Type: "requirement"}
	e3 := &model.Entity{ID: "DEC-001", Type: "decision"}
	g.AddNode(e1)
	g.AddNode(e2)
	g.AddNode(e3)

	reqs := g.NodesByType("requirement")
	if len(reqs) != 2 {
		t.Errorf("expected 2 requirements, got %d", len(reqs))
	}

	decs := g.NodesByType("decision")
	if len(decs) != 1 {
		t.Errorf("expected 1 decision, got %d", len(decs))
	}

	// Non-existent type
	none := g.NodesByType("nonexistent")
	if len(none) != 0 {
		t.Errorf("expected 0 nodes for non-existent type, got %d", len(none))
	}
}

func TestNodeCount(t *testing.T) {
	g := New()
	if g.NodeCount() != 0 {
		t.Error("expected 0 nodes initially")
	}

	g.AddNode(&model.Entity{ID: "TEST-001", Type: "test"})
	if g.NodeCount() != 1 {
		t.Error("expected 1 node after adding")
	}
}

func TestEdgeCount(t *testing.T) {
	g := New()
	if g.EdgeCount() != 0 {
		t.Error("expected 0 edges initially")
	}

	g.AddEdge(&model.Relation{From: "TEST-001", Type: "links_to", To: "TEST-002"})
	if g.EdgeCount() != 1 {
		t.Error("expected 1 edge after adding")
	}
}

func TestAllIDs(t *testing.T) {
	g := New()
	g.AddNode(&model.Entity{ID: "TEST-001", Type: "test"})
	g.AddNode(&model.Entity{ID: "TEST-002", Type: "test"})

	ids := g.AllIDs()
	if len(ids) != 2 {
		t.Errorf("expected 2 IDs, got %d", len(ids))
	}

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
	g.AddNode(&model.Entity{ID: "REQ-001", Type: "requirement"})
	g.AddNode(&model.Entity{ID: "REQ-002", Type: "requirement"})
	g.AddNode(&model.Entity{ID: "DEC-001", Type: "decision"})

	reqIDs := g.IDsByType("requirement")
	if len(reqIDs) != 2 {
		t.Errorf("expected 2 requirement IDs, got %d", len(reqIDs))
	}

	decIDs := g.IDsByType("decision")
	if len(decIDs) != 1 {
		t.Errorf("expected 1 decision ID, got %d", len(decIDs))
	}
}

func TestClear(t *testing.T) {
	g := New()
	g.AddNode(&model.Entity{ID: "TEST-001", Type: "test"})
	g.AddEdge(&model.Relation{From: "TEST-001", Type: "links_to", To: "TEST-002"})

	g.Clear()

	if g.NodeCount() != 0 {
		t.Error("expected 0 nodes after clear")
	}
	if g.EdgeCount() != 0 {
		t.Error("expected 0 edges after clear")
	}
}

func TestTraceTo(t *testing.T) {
	g := New()
	g.AddNode(&model.Entity{ID: "A", Type: "test", Properties: map[string]interface{}{"title": "A"}})
	g.AddNode(&model.Entity{ID: "B", Type: "test", Properties: map[string]interface{}{"title": "B"}})
	g.AddNode(&model.Entity{ID: "C", Type: "test", Properties: map[string]interface{}{"title": "C"}})

	g.AddEdge(&model.Relation{From: "A", Type: "links_to", To: "B"})
	g.AddEdge(&model.Relation{From: "B", Type: "links_to", To: "C"})

	result := g.TraceTo("C", 10)
	if result == nil {
		t.Fatal("expected trace result")
	}
	if result.ID != "C" {
		t.Errorf("expected root to be C, got %s", result.ID)
	}
	if len(result.Children) == 0 {
		t.Error("expected trace to have children")
	}
}

func TestFindOrphans(t *testing.T) {
	g := New()
	g.AddNode(&model.Entity{ID: "ORPHAN", Type: "test"})
	g.AddNode(&model.Entity{ID: "CONNECTED", Type: "test"})
	g.AddEdge(&model.Relation{From: "CONNECTED", Type: "links_to", To: "OTHER"})

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
	g.AddNode(&model.Entity{ID: "A", Type: "test"})
	g.AddNode(&model.Entity{ID: "B", Type: "test"})
	g.AddNode(&model.Entity{ID: "C", Type: "test"})
	g.AddNode(&model.Entity{ID: "D", Type: "test"})

	// Cluster 1: A-B
	g.AddEdge(&model.Relation{From: "A", Type: "links_to", To: "B"})

	// Cluster 2: C-D
	g.AddEdge(&model.Relation{From: "C", Type: "links_to", To: "D"})

	clusters := g.FindClusters()
	if len(clusters) != 2 {
		t.Errorf("expected 2 clusters, got %d", len(clusters))
	}
}

func TestHasCycle(t *testing.T) {
	g := New()
	g.AddNode(&model.Entity{ID: "A", Type: "test"})
	g.AddNode(&model.Entity{ID: "B", Type: "test"})
	g.AddNode(&model.Entity{ID: "C", Type: "test"})

	// Create cycle: A -> B -> C -> A
	g.AddEdge(&model.Relation{From: "A", Type: "links_to", To: "B"})
	g.AddEdge(&model.Relation{From: "B", Type: "links_to", To: "C"})
	g.AddEdge(&model.Relation{From: "C", Type: "links_to", To: "A"})

	hasCycle := g.HasCycle("A")
	if !hasCycle {
		t.Error("expected to detect cycle")
	}

	// No cycle graph
	g2 := New()
	g2.AddNode(&model.Entity{ID: "X", Type: "test"})
	g2.AddNode(&model.Entity{ID: "Y", Type: "test"})
	g2.AddEdge(&model.Relation{From: "X", Type: "links_to", To: "Y"})

	hasCycle2 := g2.HasCycle("X")
	if hasCycle2 {
		t.Error("expected no cycle in linear graph")
	}
}

func TestRelationsOfType(t *testing.T) {
	g := New()
	g.AddEdge(&model.Relation{From: "A", Type: "implements", To: "B"})
	g.AddEdge(&model.Relation{From: "C", Type: "depends_on", To: "D"})
	g.AddEdge(&model.Relation{From: "E", Type: "implements", To: "F"})

	implements := g.RelationsOfType("implements")
	if len(implements) != 2 {
		t.Errorf("expected 2 'implements' relations, got %d", len(implements))
	}

	dependsOn := g.RelationsOfType("depends_on")
	if len(dependsOn) != 1 {
		t.Errorf("expected 1 'depends_on' relation, got %d", len(dependsOn))
	}
}
