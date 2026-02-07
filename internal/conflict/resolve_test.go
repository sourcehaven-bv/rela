package conflict

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolve_Entity(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "REQ-001.md")

	content := `---
id: REQ-001
type: requirement
<<<<<<< HEAD
status: draft
priority: high
=======
status: approved
priority: low
>>>>>>> feature
title: Test
---

<<<<<<< HEAD
Current content.
=======
Incoming content.
>>>>>>> feature
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to create file: %v", err)
	}

	cf, err := ParseConflictedFile(path, nil)
	if err != nil {
		t.Fatalf("ParseConflictedFile failed: %v", err)
	}

	// Resolve with mixed choices
	resolution := &Resolution{
		PropertyChoices: map[string]Side{
			"status":   SideTheirs, // Take theirs (approved)
			"priority": SideOurs,   // Keep ours (high)
			"title":    SideOurs,   // Keep ours
		},
		ContentChoice: SideTheirs, // Take theirs content
	}

	entity, relation, err := Resolve(cf, resolution)
	if err != nil {
		t.Fatalf("Resolve failed: %v", err)
	}

	if relation != nil {
		t.Error("expected nil relation for entity file")
	}
	if entity == nil {
		t.Fatal("expected entity, got nil")
	}

	if entity.ID != "REQ-001" {
		t.Errorf("ID = %q, want REQ-001", entity.ID)
	}
	if entity.GetString("status") != "approved" {
		t.Errorf("status = %q, want approved (theirs)", entity.GetString("status"))
	}
	if entity.GetString("priority") != "high" {
		t.Errorf("priority = %q, want high (ours)", entity.GetString("priority"))
	}
	if !strings.Contains(entity.Content, "Incoming content") {
		t.Error("content should be theirs (incoming)")
	}
}

func TestResolve_Relation(t *testing.T) {
	content := `---
from: REQ-001
relation: depends-on
<<<<<<< HEAD
to: REQ-002
=======
to: REQ-003
>>>>>>> feature
---
`
	path := "/project/relations/REQ-001--depends-on--conflict.md"

	cf, err := ParseConflictedContent(path, content, nil)
	if err != nil {
		t.Fatalf("ParseConflictedContent failed: %v", err)
	}

	resolution := &Resolution{
		PropertyChoices: map[string]Side{
			"to": SideTheirs, // Take theirs (REQ-003)
		},
		ContentChoice: SideOurs,
	}

	entity, relation, err := Resolve(cf, resolution)
	if err != nil {
		t.Fatalf("Resolve failed: %v", err)
	}

	if entity != nil {
		t.Error("expected nil entity for relation file")
	}
	if relation == nil {
		t.Fatal("expected relation, got nil")
	}

	if relation.From != "REQ-001" {
		t.Errorf("From = %q, want REQ-001", relation.From)
	}
	if relation.To != "REQ-003" {
		t.Errorf("To = %q, want REQ-003 (theirs)", relation.To)
	}
}

func TestAcceptOurs(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "REQ-001.md")

	content := `---
id: REQ-001
type: requirement
<<<<<<< HEAD
status: draft
=======
status: approved
>>>>>>> feature
---
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to create file: %v", err)
	}

	cf, err := ParseConflictedFile(path, nil)
	if err != nil {
		t.Fatalf("ParseConflictedFile failed: %v", err)
	}

	resolution := AcceptOurs(cf)

	entity, _, err := Resolve(cf, resolution)
	if err != nil {
		t.Fatalf("Resolve failed: %v", err)
	}

	if entity.GetString("status") != "draft" {
		t.Errorf("status = %q, want draft (ours)", entity.GetString("status"))
	}
}

func TestAcceptTheirs(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "REQ-001.md")

	content := `---
id: REQ-001
type: requirement
<<<<<<< HEAD
status: draft
=======
status: approved
>>>>>>> feature
---
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to create file: %v", err)
	}

	cf, err := ParseConflictedFile(path, nil)
	if err != nil {
		t.Fatalf("ParseConflictedFile failed: %v", err)
	}

	resolution := AcceptTheirs(cf)

	entity, _, err := Resolve(cf, resolution)
	if err != nil {
		t.Fatalf("Resolve failed: %v", err)
	}

	if entity.GetString("status") != "approved" {
		t.Errorf("status = %q, want approved (theirs)", entity.GetString("status"))
	}
}

func TestResolve_ManualContent(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "REQ-001.md")

	content := `---
id: REQ-001
type: requirement
status: draft
---

<<<<<<< HEAD
Current content.
=======
Incoming content.
>>>>>>> feature
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to create file: %v", err)
	}

	cf, err := ParseConflictedFile(path, nil)
	if err != nil {
		t.Fatalf("ParseConflictedFile failed: %v", err)
	}

	resolution := &Resolution{
		PropertyChoices: make(map[string]Side),
		ManualContent:   "Manually edited content.",
	}

	entity, _, err := Resolve(cf, resolution)
	if err != nil {
		t.Fatalf("Resolve failed: %v", err)
	}

	if entity.Content != "Manually edited content." {
		t.Errorf("content = %q, want manual content", entity.Content)
	}
}

func TestResolveAndWrite(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "REQ-001.md")

	content := `---
id: REQ-001
type: requirement
<<<<<<< HEAD
status: draft
=======
status: approved
>>>>>>> feature
---

Content.
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to create file: %v", err)
	}

	cf, err := ParseConflictedFile(path, nil)
	if err != nil {
		t.Fatalf("ParseConflictedFile failed: %v", err)
	}

	resolution := AcceptTheirs(cf)

	err = ResolveAndWrite(cf, resolution, nil)
	if err != nil {
		t.Fatalf("ResolveAndWrite failed: %v", err)
	}

	// Read back and verify
	newContent, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read resolved file: %v", err)
	}

	if strings.Contains(string(newContent), "<<<<<<<") {
		t.Error("resolved file should not contain conflict markers")
	}
	if !strings.Contains(string(newContent), "status: approved") {
		t.Error("resolved file should contain theirs status")
	}
}

func TestRemoveConflictMarkers(t *testing.T) {
	tmpDir := t.TempDir()

	tests := []struct {
		name     string
		content  string
		keepSide Side
		expected string
	}{
		{
			name: "keep ours",
			content: `---
id: REQ-001
<<<<<<< HEAD
status: draft
=======
status: approved
>>>>>>> feature
---
`,
			keepSide: SideOurs,
			expected: "status: draft",
		},
		{
			name: "keep theirs",
			content: `---
id: REQ-001
<<<<<<< HEAD
status: draft
=======
status: approved
>>>>>>> feature
---
`,
			keepSide: SideTheirs,
			expected: "status: approved",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(tmpDir, tt.name+".md")
			if err := os.WriteFile(path, []byte(tt.content), 0644); err != nil {
				t.Fatalf("failed to create file: %v", err)
			}

			err := RemoveConflictMarkers(path, tt.keepSide)
			if err != nil {
				t.Fatalf("RemoveConflictMarkers failed: %v", err)
			}

			newContent, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("failed to read file: %v", err)
			}

			if strings.Contains(string(newContent), "<<<<<<<") {
				t.Error("should not contain conflict markers")
			}
			if !strings.Contains(string(newContent), tt.expected) {
				t.Errorf("should contain %q", tt.expected)
			}
		})
	}
}

func TestCollectPropertyKeys(t *testing.T) {
	a := map[string]interface{}{"foo": 1, "bar": 2}
	b := map[string]interface{}{"bar": 3, "baz": 4}

	keys := collectPropertyKeys(a, b)

	if len(keys) != 3 {
		t.Errorf("expected 3 keys, got %d", len(keys))
	}

	keyMap := make(map[string]bool)
	for _, k := range keys {
		keyMap[k] = true
	}

	if !keyMap["foo"] || !keyMap["bar"] || !keyMap["baz"] {
		t.Error("missing expected keys")
	}
}
