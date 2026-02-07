package conflict

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/project"
)

func TestFindMarkers(t *testing.T) {
	content := `---
id: REQ-001
<<<<<<< HEAD
status: draft
=======
status: approved
>>>>>>> feature-branch
---

Content here.
`

	markers := FindMarkers(content)

	if len(markers) != 1 {
		t.Fatalf("expected 1 marker, got %d", len(markers))
	}

	m := markers[0]
	if m.StartLine != 3 {
		t.Errorf("StartLine = %d, want 3", m.StartLine)
	}
	if m.MidLine != 5 {
		t.Errorf("MidLine = %d, want 5", m.MidLine)
	}
	if m.EndLine != 7 {
		t.Errorf("EndLine = %d, want 7", m.EndLine)
	}
	if m.OursRef != "HEAD" {
		t.Errorf("OursRef = %q, want HEAD", m.OursRef)
	}
	if m.TheirsRef != "feature-branch" {
		t.Errorf("TheirsRef = %q, want feature-branch", m.TheirsRef)
	}
}

func TestFindMarkers_Multiple(t *testing.T) {
	content := `---
<<<<<<< HEAD
id: REQ-001
=======
id: REQ-002
>>>>>>> branch
<<<<<<< HEAD
status: draft
=======
status: done
>>>>>>> branch
---
`

	markers := FindMarkers(content)

	if len(markers) != 2 {
		t.Fatalf("expected 2 markers, got %d", len(markers))
	}
}

func TestFindMarkers_NoConflicts(t *testing.T) {
	content := `---
id: REQ-001
status: draft
---

Normal content.
`

	markers := FindMarkers(content)

	if len(markers) != 0 {
		t.Errorf("expected 0 markers, got %d", len(markers))
	}
}

func TestHasConflicts(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected bool
	}{
		{
			name:     "no conflicts",
			content:  "---\nid: REQ-001\n---\n",
			expected: false,
		},
		{
			name:     "has conflicts",
			content:  "---\n<<<<<<< HEAD\nstatus: x\n=======\nstatus: y\n>>>>>>> b\n---\n",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := HasConflicts(tt.content)
			if got != tt.expected {
				t.Errorf("HasConflicts() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestDetectInFile(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a conflicted file
	conflictedPath := filepath.Join(tmpDir, "conflicted.md")
	conflictedContent := `---
id: REQ-001
<<<<<<< HEAD
status: draft
=======
status: approved
>>>>>>> feature
---
`
	if err := os.WriteFile(conflictedPath, []byte(conflictedContent), 0644); err != nil {
		t.Fatalf("failed to create file: %v", err)
	}

	// Create a normal file
	normalPath := filepath.Join(tmpDir, "normal.md")
	normalContent := `---
id: REQ-002
status: draft
---
`
	if err := os.WriteFile(normalPath, []byte(normalContent), 0644); err != nil {
		t.Fatalf("failed to create file: %v", err)
	}

	// Test conflicted file
	cf, err := DetectInFile(conflictedPath)
	if err != nil {
		t.Fatalf("DetectInFile failed: %v", err)
	}
	if cf == nil {
		t.Error("expected conflict to be detected")
	}
	if cf.Path != conflictedPath {
		t.Errorf("Path = %q, want %q", cf.Path, conflictedPath)
	}
	if len(cf.Markers) != 1 {
		t.Errorf("expected 1 marker, got %d", len(cf.Markers))
	}

	// Test normal file
	cf, err = DetectInFile(normalPath)
	if err != nil {
		t.Fatalf("DetectInFile failed: %v", err)
	}
	if cf != nil {
		t.Error("expected no conflict in normal file")
	}
}

func TestDetectAll(t *testing.T) {
	tmpDir := t.TempDir()

	ctx := &project.Context{
		Root:         tmpDir,
		EntitiesDir:  filepath.Join(tmpDir, "entities"),
		RelationsDir: filepath.Join(tmpDir, "relations"),
	}

	// Create directories
	reqDir := filepath.Join(ctx.EntitiesDir, "requirement")
	if err := os.MkdirAll(reqDir, 0755); err != nil {
		t.Fatalf("failed to create dir: %v", err)
	}
	if err := os.MkdirAll(ctx.RelationsDir, 0755); err != nil {
		t.Fatalf("failed to create dir: %v", err)
	}

	// Create a conflicted entity
	conflictedEntity := `---
id: REQ-001
<<<<<<< HEAD
status: draft
=======
status: approved
>>>>>>> feature
---
`
	if err := os.WriteFile(filepath.Join(reqDir, "REQ-001.md"), []byte(conflictedEntity), 0644); err != nil {
		t.Fatalf("failed to create file: %v", err)
	}

	// Create a normal entity
	normalEntity := `---
id: REQ-002
status: draft
---
`
	if err := os.WriteFile(filepath.Join(reqDir, "REQ-002.md"), []byte(normalEntity), 0644); err != nil {
		t.Fatalf("failed to create file: %v", err)
	}

	// Create a conflicted relation
	conflictedRelation := `---
from: REQ-001
relation: depends-on
<<<<<<< HEAD
to: REQ-002
=======
to: REQ-003
>>>>>>> feature
---
`
	if err := os.WriteFile(filepath.Join(ctx.RelationsDir, "conflict.md"), []byte(conflictedRelation), 0644); err != nil {
		t.Fatalf("failed to create file: %v", err)
	}

	result, err := DetectAll(ctx)
	if err != nil {
		t.Fatalf("DetectAll failed: %v", err)
	}

	if len(result.Files) != 2 {
		t.Errorf("expected 2 conflicted files, got %d", len(result.Files))
	}
}

func TestInferEntityType(t *testing.T) {
	tests := []struct {
		path        string
		entitiesDir string
		expected    string
	}{
		{
			path:        "/project/entities/requirement/REQ-001.md",
			entitiesDir: "/project/entities",
			expected:    "requirement",
		},
		{
			path:        "/project/entities/decision/DEC-001.md",
			entitiesDir: "/project/entities",
			expected:    "decision",
		},
		{
			path:        "/project/entities/single.md",
			entitiesDir: "/project/entities",
			expected:    "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			got := inferEntityType(tt.path, tt.entitiesDir)
			if got != tt.expected {
				t.Errorf("inferEntityType() = %q, want %q", got, tt.expected)
			}
		})
	}
}
