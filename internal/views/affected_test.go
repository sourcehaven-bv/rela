package views

import (
	"testing"
)

func TestAffectedRootsDirectChange(t *testing.T) {
	g, meta := setupDepsTestGraph()
	engine := NewEngine(g, meta)
	view := makeDocView()

	// Changing DOC-001 itself should affect DOC-001
	affected, err := engine.AffectedRoots(view, []string{"DOC-001"}, []string{"DOC-001", "DOC-002"})
	if err != nil {
		t.Fatalf("AffectedRoots failed: %v", err)
	}

	if len(affected) != 1 || affected[0].ID != "DOC-001" {
		t.Errorf("expected [DOC-001], got %v", entityIDs(affected))
	}
}

func TestAffectedRootsTransitiveChange(t *testing.T) {
	g, meta := setupDepsTestGraph()
	engine := NewEngine(g, meta)
	view := makeDocView()

	// COMP-001 is used by both DOC-001 (via SEC-001) and DOC-002 (via SEC-003)
	affected, err := engine.AffectedRoots(view, []string{"COMP-001"}, []string{"DOC-001", "DOC-002"})
	if err != nil {
		t.Fatalf("AffectedRoots failed: %v", err)
	}

	if len(affected) != 2 {
		t.Errorf("expected 2 affected roots, got %d: %v", len(affected), entityIDs(affected))
	}
}

func TestAffectedRootsNoMatch(t *testing.T) {
	g, meta := setupDepsTestGraph()
	engine := NewEngine(g, meta)
	view := makeDocView()

	// COMP-002 is only used by DOC-001
	affected, err := engine.AffectedRoots(view, []string{"COMP-002"}, []string{"DOC-002"})
	if err != nil {
		t.Fatalf("AffectedRoots failed: %v", err)
	}

	if len(affected) != 0 {
		t.Errorf("expected no affected roots, got %v", entityIDs(affected))
	}
}

func TestAffectedRootsChangedEntityNotInGraph(t *testing.T) {
	g, meta := setupDepsTestGraph()
	engine := NewEngine(g, meta)
	view := makeDocView()

	affected, err := engine.AffectedRoots(view, []string{"NONEXISTENT"}, []string{"DOC-001"})
	if err != nil {
		t.Fatalf("AffectedRoots failed: %v", err)
	}

	if len(affected) != 0 {
		t.Errorf("expected no affected roots, got %v", entityIDs(affected))
	}
}

func TestAffectedRootsMissingRoot(t *testing.T) {
	g, meta := setupDepsTestGraph()
	engine := NewEngine(g, meta)
	view := makeDocView()

	affected, err := engine.AffectedRoots(view, []string{"SEC-001"}, []string{"NONEXISTENT", "DOC-001"})
	if err != nil {
		t.Fatalf("AffectedRoots failed: %v", err)
	}

	// DOC-001 contains SEC-001, so it should be affected
	if len(affected) != 1 || affected[0].ID != "DOC-001" {
		t.Errorf("expected [DOC-001], got %v", entityIDs(affected))
	}
}

func TestAffectedRootsEmptyChanged(t *testing.T) {
	g, meta := setupDepsTestGraph()
	engine := NewEngine(g, meta)
	view := makeDocView()

	affected, err := engine.AffectedRoots(view, []string{}, []string{"DOC-001", "DOC-002"})
	if err != nil {
		t.Fatalf("AffectedRoots failed: %v", err)
	}

	if len(affected) != 0 {
		t.Errorf("expected no affected roots, got %v", entityIDs(affected))
	}
}

func TestAffectedRootsSortedOutput(t *testing.T) {
	g, meta := setupDepsTestGraph()
	engine := NewEngine(g, meta)
	view := makeDocView()

	// COMP-001 is shared, both roots affected
	affected, err := engine.AffectedRoots(view, []string{"COMP-001"}, []string{"DOC-002", "DOC-001"})
	if err != nil {
		t.Fatalf("AffectedRoots failed: %v", err)
	}

	for i := 1; i < len(affected); i++ {
		if affected[i].ID < affected[i-1].ID {
			t.Errorf("output not sorted: %v", entityIDs(affected))
			break
		}
	}
}

func TestAffectedRootsSectionChange(t *testing.T) {
	g, meta := setupDepsTestGraph()
	engine := NewEngine(g, meta)
	view := makeDocView()

	// SEC-003 is only in DOC-002
	affected, err := engine.AffectedRoots(view, []string{"SEC-003"}, []string{"DOC-001", "DOC-002"})
	if err != nil {
		t.Fatalf("AffectedRoots failed: %v", err)
	}

	if len(affected) != 1 || affected[0].ID != "DOC-002" {
		t.Errorf("expected [DOC-002], got %v", entityIDs(affected))
	}
}
