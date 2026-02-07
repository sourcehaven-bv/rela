package conflict

import (
	"os"
	"path/filepath"
	"testing"
)

func TestExtractSides(t *testing.T) {
	content := `---
id: REQ-001
<<<<<<< HEAD
status: draft
priority: high
=======
status: approved
priority: low
>>>>>>> feature-branch
title: Test
---

Content here.
`

	markers := FindMarkers(content)
	ours, theirs := ExtractSides(content, markers)

	// Check ours side
	expectedOurs := `---
id: REQ-001
status: draft
priority: high
title: Test
---

Content here.
`
	if ours != expectedOurs {
		t.Errorf("ours =\n%s\nwant:\n%s", ours, expectedOurs)
	}

	// Check theirs side
	expectedTheirs := `---
id: REQ-001
status: approved
priority: low
title: Test
---

Content here.
`
	if theirs != expectedTheirs {
		t.Errorf("theirs =\n%s\nwant:\n%s", theirs, expectedTheirs)
	}
}

func TestExtractSides_MultipleConflicts(t *testing.T) {
	content := `---
<<<<<<< HEAD
id: REQ-001
=======
id: REQ-002
>>>>>>> branch
type: requirement
<<<<<<< HEAD
status: draft
=======
status: done
>>>>>>> branch
---
`

	markers := FindMarkers(content)
	ours, theirs := ExtractSides(content, markers)

	if !contains(ours, "id: REQ-001") {
		t.Error("ours should contain 'id: REQ-001'")
	}
	if !contains(ours, "status: draft") {
		t.Error("ours should contain 'status: draft'")
	}
	if !contains(theirs, "id: REQ-002") {
		t.Error("theirs should contain 'id: REQ-002'")
	}
	if !contains(theirs, "status: done") {
		t.Error("theirs should contain 'status: done'")
	}
}

func TestExtractSides_ConflictInContent(t *testing.T) {
	content := `---
id: REQ-001
status: draft
---

# Description

<<<<<<< HEAD
This is the current content.
=======
This is the incoming content.
>>>>>>> feature
`

	markers := FindMarkers(content)
	ours, theirs := ExtractSides(content, markers)

	if !contains(ours, "This is the current content.") {
		t.Error("ours should contain current content")
	}
	if contains(ours, "This is the incoming content.") {
		t.Error("ours should not contain incoming content")
	}
	if !contains(theirs, "This is the incoming content.") {
		t.Error("theirs should contain incoming content")
	}
	if contains(theirs, "This is the current content.") {
		t.Error("theirs should not contain current content")
	}
}

func TestParseConflictedFile(t *testing.T) {
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
priority: medium
>>>>>>> feature
title: Test Requirement
---

# Description

Test content.
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to create file: %v", err)
	}

	cf, err := ParseConflictedFile(path, nil)
	if err != nil {
		t.Fatalf("ParseConflictedFile failed: %v", err)
	}

	if cf.Path != path {
		t.Errorf("Path = %q, want %q", cf.Path, path)
	}

	if cf.Ours == nil {
		t.Fatal("Ours is nil")
	}
	if cf.Theirs == nil {
		t.Fatal("Theirs is nil")
	}

	// Check ours entity
	if cf.Ours.Entity == nil {
		t.Fatal("Ours.Entity is nil")
	}
	if cf.Ours.Entity.ID != "REQ-001" {
		t.Errorf("Ours.Entity.ID = %q, want REQ-001", cf.Ours.Entity.ID)
	}
	if cf.Ours.Entity.GetString("status") != "draft" {
		t.Errorf("Ours status = %q, want draft", cf.Ours.Entity.GetString("status"))
	}
	if cf.Ours.Entity.GetString("priority") != "high" {
		t.Errorf("Ours priority = %q, want high", cf.Ours.Entity.GetString("priority"))
	}

	// Check theirs entity
	if cf.Theirs.Entity == nil {
		t.Fatal("Theirs.Entity is nil")
	}
	if cf.Theirs.Entity.GetString("status") != "approved" {
		t.Errorf("Theirs status = %q, want approved", cf.Theirs.Entity.GetString("status"))
	}
	if cf.Theirs.Entity.GetString("priority") != "medium" {
		t.Errorf("Theirs priority = %q, want medium", cf.Theirs.Entity.GetString("priority"))
	}

	// Common properties should be the same
	if cf.Ours.Entity.GetString("title") != cf.Theirs.Entity.GetString("title") {
		t.Error("title should be the same on both sides")
	}
}

func TestParseConflictedContent_Relation(t *testing.T) {
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

	// Use a relation-style path
	path := "/project/relations/REQ-001--depends-on--conflict.md"

	cf, err := ParseConflictedContent(path, content, nil)
	if err != nil {
		t.Fatalf("ParseConflictedContent failed: %v", err)
	}

	if cf.Ours.Relation == nil {
		t.Fatal("Ours.Relation is nil")
	}
	if cf.Theirs.Relation == nil {
		t.Fatal("Theirs.Relation is nil")
	}

	if cf.Ours.Relation.From != "REQ-001" {
		t.Errorf("Ours.Relation.From = %q, want REQ-001", cf.Ours.Relation.From)
	}
	if cf.Ours.Relation.To != "REQ-002" {
		t.Errorf("Ours.Relation.To = %q, want REQ-002", cf.Ours.Relation.To)
	}
	if cf.Theirs.Relation.To != "REQ-003" {
		t.Errorf("Theirs.Relation.To = %q, want REQ-003", cf.Theirs.Relation.To)
	}
}

func TestAnalyzeConflict(t *testing.T) {
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
priority: high
>>>>>>> feature
title: Same Title
---

<<<<<<< HEAD
Current content.
=======
Different content.
>>>>>>> feature
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to create file: %v", err)
	}

	cf, err := ParseConflictedFile(path, nil)
	if err != nil {
		t.Fatalf("ParseConflictedFile failed: %v", err)
	}

	info := AnalyzeConflict(cf)

	// Check property diffs
	statusDiff := findPropertyDiff(info.PropertyDiffs, "status")
	if statusDiff == nil {
		t.Fatal("status diff not found")
	}
	if statusDiff.IsSame {
		t.Error("status should be different")
	}
	if statusDiff.OursValue != "draft" {
		t.Errorf("status ours = %v, want draft", statusDiff.OursValue)
	}
	if statusDiff.TheirsValue != "approved" {
		t.Errorf("status theirs = %v, want approved", statusDiff.TheirsValue)
	}

	priorityDiff := findPropertyDiff(info.PropertyDiffs, "priority")
	if priorityDiff == nil {
		t.Fatal("priority diff not found")
	}
	if !priorityDiff.IsSame {
		t.Error("priority should be the same")
	}

	// Check content diff
	if info.ContentSame {
		t.Error("content should be different")
	}
}

func TestIsRelationFile(t *testing.T) {
	tests := []struct {
		path     string
		expected bool
	}{
		{"/project/relations/REQ-001--depends-on--REQ-002.md", true},
		{"/project/relations/DEC-001--addresses--REQ-001.md", true},
		{"/project/entities/requirement/REQ-001.md", false},
		{"/project/relations/invalid.md", false},
		{"/project/relations/one--two.md", false},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			got := isRelationFile(tt.path)
			if got != tt.expected {
				t.Errorf("isRelationFile(%q) = %v, want %v", tt.path, got, tt.expected)
			}
		})
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func findPropertyDiff(diffs []PropertyDiff, prop string) *PropertyDiff {
	for i := range diffs {
		if diffs[i].Property == prop {
			return &diffs[i]
		}
	}
	return nil
}
