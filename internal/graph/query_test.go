package graph

import (
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/model"
)

func TestTraceBoth(t *testing.T) {
	// Create a simple graph:
	// RISK-001 -> threatens -> AST-001 <- implements <- CTRL-001
	g := New()

	risk := &model.Entity{ID: "RISK-001", Type: "risk", Properties: map[string]interface{}{"title": "Data Breach Risk"}}
	asset := &model.Entity{ID: "AST-001", Type: "asset", Properties: map[string]interface{}{"title": "Customer Database"}}
	control := &model.Entity{ID: "CTRL-001", Type: "control", Properties: map[string]interface{}{"title": "Encryption"}}

	g.AddNode(risk)
	g.AddNode(asset)
	g.AddNode(control)

	g.AddEdge(&model.Relation{From: "RISK-001", To: "AST-001", Type: "threatens"})
	g.AddEdge(&model.Relation{From: "CTRL-001", To: "AST-001", Type: "implements"})

	// Trace from AST-001 should show both incoming relations
	result := g.TraceBoth("AST-001", 2)

	if result == nil {
		t.Fatal("TraceBoth returned nil")
	}

	if result.ID != "AST-001" {
		t.Errorf("Root ID = %q, want %q", result.ID, "AST-001")
	}

	// Should have 2 children (RISK-001 and CTRL-001, both incoming)
	if len(result.Children) != 2 {
		t.Errorf("Number of children = %d, want 2", len(result.Children))
	}

	// Verify all children are marked as incoming
	for _, child := range result.Children {
		if !child.Incoming {
			t.Errorf("Child %s should be marked as incoming", child.ID)
		}
	}
}

func TestTraceBothWithOutgoing(t *testing.T) {
	// Create a graph with both directions:
	// RISK-001 -> threatens -> AST-001 -> contains -> DATA-001
	g := New()

	risk := &model.Entity{ID: "RISK-001", Type: "risk", Properties: map[string]interface{}{"title": "Risk"}}
	asset := &model.Entity{ID: "AST-001", Type: "asset", Properties: map[string]interface{}{"title": "Asset"}}
	data := &model.Entity{ID: "DATA-001", Type: "data", Properties: map[string]interface{}{"title": "Data"}}

	g.AddNode(risk)
	g.AddNode(asset)
	g.AddNode(data)

	g.AddEdge(&model.Relation{From: "RISK-001", To: "AST-001", Type: "threatens"})
	g.AddEdge(&model.Relation{From: "AST-001", To: "DATA-001", Type: "contains"})

	result := g.TraceBoth("AST-001", 2)

	if result == nil {
		t.Fatal("TraceBoth returned nil")
	}

	// Should have 2 children: RISK-001 (incoming) and DATA-001 (outgoing)
	if len(result.Children) != 2 {
		t.Errorf("Number of children = %d, want 2", len(result.Children))
	}

	incomingCount := 0
	outgoingCount := 0
	for _, child := range result.Children {
		if child.Incoming {
			incomingCount++
			if child.ID != "RISK-001" {
				t.Errorf("Incoming child ID = %q, want RISK-001", child.ID)
			}
		} else {
			outgoingCount++
			if child.ID != "DATA-001" {
				t.Errorf("Outgoing child ID = %q, want DATA-001", child.ID)
			}
		}
	}

	if incomingCount != 1 {
		t.Errorf("Incoming count = %d, want 1", incomingCount)
	}
	if outgoingCount != 1 {
		t.Errorf("Outgoing count = %d, want 1", outgoingCount)
	}
}

// TestTraceFromIncludesIncoming tests that TraceFrom follows both outgoing AND incoming edges.
// Bug #5: trace from GOAL-001 should show EPIC-001 which points TO it via contributesTo.
func TestTraceFromIncludesIncoming(t *testing.T) {
	// Create a graph matching the bug report:
	// GOAL-001 <- contributesTo <- EPIC-001 <- partOfEpic <- FEAT-001
	// (arrows represent edge direction: FEAT-001 --partOfEpic--> EPIC-001 --contributesTo--> GOAL-001)
	g := New()

	goal := &model.Entity{ID: "GOAL-001", Type: "goal", Properties: map[string]interface{}{"title": "Increase customer retention by 15%"}}
	epic := &model.Entity{ID: "EPIC-001", Type: "epic", Properties: map[string]interface{}{"title": "Improve onboarding experience"}}
	feat := &model.Entity{ID: "FEAT-001", Type: "feature", Properties: map[string]interface{}{"title": "Welcome wizard"}}

	g.AddNode(goal)
	g.AddNode(epic)
	g.AddNode(feat)

	// EPIC-001 --contributesTo--> GOAL-001
	g.AddEdge(&model.Relation{From: "EPIC-001", To: "GOAL-001", Type: "contributesTo"})
	// FEAT-001 --partOfEpic--> EPIC-001
	g.AddEdge(&model.Relation{From: "FEAT-001", To: "EPIC-001", Type: "partOfEpic"})

	// trace from GOAL-001 should show EPIC-001 and FEAT-001
	result := g.TraceFrom("GOAL-001", 0)

	if result == nil {
		t.Fatal("TraceFrom returned nil")
	}

	if result.ID != "GOAL-001" {
		t.Errorf("Root ID = %q, want %q", result.ID, "GOAL-001")
	}

	// Should have at least 1 child (EPIC-001 via incoming contributesTo)
	if len(result.Children) == 0 {
		t.Fatal("TraceFrom should include incoming relations (EPIC-001 contributesTo GOAL-001)")
	}

	// Collect all reachable IDs
	reachable := make(map[string]bool)
	var collectIDs func(r *model.TraceResult)
	collectIDs = func(r *model.TraceResult) {
		reachable[r.ID] = true
		for _, child := range r.Children {
			collectIDs(child)
		}
	}
	collectIDs(result)

	if !reachable["EPIC-001"] {
		t.Error("TraceFrom GOAL-001 should include EPIC-001 (incoming contributesTo)")
	}
	if !reachable["FEAT-001"] {
		t.Error("TraceFrom GOAL-001 should include FEAT-001 (transitive via EPIC-001)")
	}
}

// TestFindPathBidirectional tests that FindPath finds paths regardless of edge direction.
// Bug #6: trace path GOAL-001 TASK-001 should find the path through EPIC-001 and FEAT-001.
func TestFindPathBidirectional(t *testing.T) {
	// Create a graph:
	// GOAL-001 <- contributesTo <- EPIC-001 <- partOfEpic <- FEAT-001 <- implements <- TASK-001
	g := New()

	goal := &model.Entity{ID: "GOAL-001", Type: "goal", Properties: map[string]interface{}{"title": "Goal"}}
	epic := &model.Entity{ID: "EPIC-001", Type: "epic", Properties: map[string]interface{}{"title": "Epic"}}
	feat := &model.Entity{ID: "FEAT-001", Type: "feature", Properties: map[string]interface{}{"title": "Feature"}}
	task := &model.Entity{ID: "TASK-001", Type: "task", Properties: map[string]interface{}{"title": "Task"}}

	g.AddNode(goal)
	g.AddNode(epic)
	g.AddNode(feat)
	g.AddNode(task)

	// All edges point "up" toward the goal
	g.AddEdge(&model.Relation{From: "EPIC-001", To: "GOAL-001", Type: "contributesTo"})
	g.AddEdge(&model.Relation{From: "FEAT-001", To: "EPIC-001", Type: "partOfEpic"})
	g.AddEdge(&model.Relation{From: "TASK-001", To: "FEAT-001", Type: "implements"})

	// Find path from GOAL-001 to TASK-001 (against edge direction)
	path := g.FindPath("GOAL-001", "TASK-001")

	if path == nil {
		t.Fatal("FindPath should find path from GOAL-001 to TASK-001 (traversing against edge direction)")
	}

	// Verify path contains all expected nodes
	pathIDs := make([]string, len(path))
	for i, step := range path {
		pathIDs[i] = step.ID
	}

	if len(path) != 4 {
		t.Errorf("Path length = %d, want 4 (GOAL-001 -> EPIC-001 -> FEAT-001 -> TASK-001), got %v", len(path), pathIDs)
	}

	// First should be GOAL-001, last should be TASK-001
	if path[0].ID != "GOAL-001" {
		t.Errorf("Path start = %q, want GOAL-001", path[0].ID)
	}
	if path[len(path)-1].ID != "TASK-001" {
		t.Errorf("Path end = %q, want TASK-001", path[len(path)-1].ID)
	}
}

// TestFindPathBothDirections tests that FindPath works in both directions.
func TestFindPathBothDirections(t *testing.T) {
	g := New()

	a := &model.Entity{ID: "A", Type: "node", Properties: map[string]interface{}{"title": "A"}}
	b := &model.Entity{ID: "B", Type: "node", Properties: map[string]interface{}{"title": "B"}}
	c := &model.Entity{ID: "C", Type: "node", Properties: map[string]interface{}{"title": "C"}}

	g.AddNode(a)
	g.AddNode(b)
	g.AddNode(c)

	// A -> B -> C
	g.AddEdge(&model.Relation{From: "A", To: "B", Type: "link"})
	g.AddEdge(&model.Relation{From: "B", To: "C", Type: "link"})

	// Forward direction should work
	pathForward := g.FindPath("A", "C")
	if pathForward == nil {
		t.Error("FindPath A -> C should succeed")
	}

	// Backward direction should also work (against edge direction)
	pathBackward := g.FindPath("C", "A")
	if pathBackward == nil {
		t.Error("FindPath C -> A should succeed (traversing against edge direction)")
	}
}
