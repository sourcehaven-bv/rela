package views

import (
	"testing"
)

func TestCollectDepsSingleRoot(t *testing.T) {
	g, meta := setupDepsTestGraph()
	engine := NewEngine(g, meta)
	view := makeDocView()

	ids, err := engine.CollectDeps(view, []string{"DOC-001"})
	if err != nil {
		t.Fatalf("CollectDeps failed: %v", err)
	}

	// DOC-001 contains SEC-001, SEC-002; SEC-001 describes COMP-001; SEC-002 describes COMP-002
	expected := map[string]bool{
		"DOC-001":  true,
		"SEC-001":  true,
		"SEC-002":  true,
		"COMP-001": true,
		"COMP-002": true,
	}

	if len(ids) != len(expected) {
		t.Errorf("got %d IDs, want %d: %v", len(ids), len(expected), ids)
	}
	for _, id := range ids {
		if !expected[id] {
			t.Errorf("unexpected ID in result: %s", id)
		}
	}
}

func TestCollectDepsMultipleRoots(t *testing.T) {
	g, meta := setupDepsTestGraph()
	engine := NewEngine(g, meta)
	view := makeDocView()

	ids, err := engine.CollectDeps(view, []string{"DOC-001", "DOC-002"})
	if err != nil {
		t.Fatalf("CollectDeps failed: %v", err)
	}

	// Union of both documents' deps
	expected := map[string]bool{
		"DOC-001":  true,
		"DOC-002":  true,
		"SEC-001":  true,
		"SEC-002":  true,
		"SEC-003":  true,
		"COMP-001": true,
		"COMP-002": true,
	}

	if len(ids) != len(expected) {
		t.Errorf("got %d IDs, want %d: %v", len(ids), len(expected), ids)
	}
	for _, id := range ids {
		if !expected[id] {
			t.Errorf("unexpected ID in result: %s", id)
		}
	}
}

func TestCollectDepsDeduplication(t *testing.T) {
	g, meta := setupDepsTestGraph()
	engine := NewEngine(g, meta)
	view := makeDocView()

	ids, err := engine.CollectDeps(view, []string{"DOC-001", "DOC-002"})
	if err != nil {
		t.Fatalf("CollectDeps failed: %v", err)
	}

	// COMP-001 is shared between DOC-001 and DOC-002 — should appear only once
	count := 0
	for _, id := range ids {
		if id == "COMP-001" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("COMP-001 appears %d times, want 1", count)
	}
}

func TestCollectDepsSortedOutput(t *testing.T) {
	g, meta := setupDepsTestGraph()
	engine := NewEngine(g, meta)
	view := makeDocView()

	ids, err := engine.CollectDeps(view, []string{"DOC-001"})
	if err != nil {
		t.Fatalf("CollectDeps failed: %v", err)
	}

	for i := 1; i < len(ids); i++ {
		if ids[i] < ids[i-1] {
			t.Errorf("output not sorted: %v", ids)
			break
		}
	}
}

func TestCollectDepsRootNotFound(t *testing.T) {
	g, meta := setupDepsTestGraph()
	engine := NewEngine(g, meta)
	view := makeDocView()

	ids, err := engine.CollectDeps(view, []string{"NONEXISTENT"})
	if err != nil {
		t.Fatalf("CollectDeps failed: %v", err)
	}

	if len(ids) != 0 {
		t.Errorf("expected empty result for missing root, got %v", ids)
	}
}

func TestCollectDepsWrongEntryType(t *testing.T) {
	g, meta := setupDepsTestGraph()
	engine := NewEngine(g, meta)
	view := makeDocView()

	// COMP-001 exists but is not a document
	ids, err := engine.CollectDeps(view, []string{"COMP-001"})
	if err != nil {
		t.Fatalf("CollectDeps failed: %v", err)
	}

	if len(ids) != 0 {
		t.Errorf("expected empty result for wrong type root, got %v", ids)
	}
}

func TestCollectDepsEmptyRoots(t *testing.T) {
	g, meta := setupDepsTestGraph()
	engine := NewEngine(g, meta)
	view := makeDocView()

	ids, err := engine.CollectDeps(view, []string{})
	if err != nil {
		t.Fatalf("CollectDeps failed: %v", err)
	}

	if len(ids) != 0 {
		t.Errorf("expected empty result for empty roots, got %v", ids)
	}
}

func TestCollectDepsIncludesRootWhenEntryExcluded(t *testing.T) {
	g, meta := setupDepsTestGraph()
	engine := NewEngine(g, meta)

	// View with include_entry: false and include_content: true
	// This triggers enrichResult to set result.Entry = nil
	view := ViewDef{
		Entry: EntryDef{
			Type:      "document",
			Parameter: "doc_id",
		},
		Output: OutputDef{
			IncludeEntry:   false,
			IncludeContent: true,
		},
		Traverse: []TraverseRule{
			{
				From:      "entry",
				Follow:    "contains",
				CollectAs: "sections",
			},
		},
	}

	ids, err := engine.CollectDeps(view, []string{"DOC-001"})
	if err != nil {
		t.Fatalf("CollectDeps failed: %v", err)
	}

	// DOC-001 must still appear even though include_entry is false
	found := false
	for _, id := range ids {
		if id == "DOC-001" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("root entity DOC-001 missing from deps when include_entry=false: %v", ids)
	}
}

func TestCollectDepsMixedValidAndInvalidRoots(t *testing.T) {
	g, meta := setupDepsTestGraph()
	engine := NewEngine(g, meta)
	view := makeDocView()

	// DOC-001 is valid, NONEXISTENT is not, COMP-001 is wrong type
	ids, err := engine.CollectDeps(view, []string{"DOC-001", "NONEXISTENT", "COMP-001"})
	if err != nil {
		t.Fatalf("CollectDeps failed: %v", err)
	}

	// Should only contain deps from DOC-001
	expected := map[string]bool{
		"DOC-001":  true,
		"SEC-001":  true,
		"SEC-002":  true,
		"COMP-001": true,
		"COMP-002": true,
	}

	if len(ids) != len(expected) {
		t.Errorf("got %d IDs, want %d: %v", len(ids), len(expected), ids)
	}
}
