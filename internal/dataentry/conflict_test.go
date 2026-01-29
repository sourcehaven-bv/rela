package dataentry

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestComputeFieldConflicts_AllStatuses(t *testing.T) {
	base := &ParsedVersion{
		Properties: map[string]string{
			"title":    "Original",
			"status":   "open",
			"priority": "medium",
			"assignee": "alice",
			"tags":     "v1",
		},
	}
	ours := &ParsedVersion{
		Properties: map[string]string{
			"title":    "Original",    // unchanged
			"status":   "in-progress", // we changed
			"priority": "high",        // both changed differently
			"assignee": "bob",         // both changed to same
			"tags":     "v1",          // they changed, we didn't
		},
	}
	theirs := &ParsedVersion{
		Properties: map[string]string{
			"title":    "Original",
			"status":   "open",     // they didn't change
			"priority": "critical", // both changed differently
			"assignee": "bob",      // both changed to same
			"tags":     "v2",       // they changed
		},
	}

	fields := []string{"title", "status", "priority", "assignee", "tags"}
	result := ComputeFieldConflicts(base, ours, theirs, fields)

	if len(result) != 5 {
		t.Fatalf("expected 5 fields, got %d", len(result))
	}

	expected := map[string]string{
		"title":    "unchanged",
		"status":   "auto-ours",
		"priority": "conflict",
		"assignee": "auto-ours", // both changed to same = no conflict
		"tags":     "auto-theirs",
	}

	for _, f := range result {
		want, ok := expected[f.Property]
		if !ok {
			t.Errorf("unexpected field %q", f.Property)
			continue
		}
		if f.Status != want {
			t.Errorf("field %q: got status %q, want %q", f.Property, f.Status, want)
		}
	}
}

func TestComputeFieldConflicts_NilBase(t *testing.T) {
	// New file — no common ancestor
	ours := &ParsedVersion{
		Properties: map[string]string{"title": "A", "status": "open"},
	}
	theirs := &ParsedVersion{
		Properties: map[string]string{"title": "B", "status": "open"},
	}

	result := ComputeFieldConflicts(nil, ours, theirs, []string{"title", "status"})

	if len(result) != 2 {
		t.Fatalf("expected 2 fields, got %d", len(result))
	}

	// title: both changed from "" differently
	if result[0].Status != "conflict" {
		t.Errorf("title: got %q, want conflict", result[0].Status)
	}
	// status: both set to same value
	if result[1].Status != "auto-ours" {
		t.Errorf("status: got %q, want auto-ours", result[1].Status)
	}
}

func TestComputeBodyConflict_NoConflict(t *testing.T) {
	tests := []struct {
		name   string
		base   *ParsedVersion
		ours   *ParsedVersion
		theirs *ParsedVersion
	}{
		{
			name:   "all identical",
			base:   &ParsedVersion{Body: "hello"},
			ours:   &ParsedVersion{Body: "hello"},
			theirs: &ParsedVersion{Body: "hello"},
		},
		{
			name:   "only ours changed",
			base:   &ParsedVersion{Body: "hello"},
			ours:   &ParsedVersion{Body: "hello world"},
			theirs: &ParsedVersion{Body: "hello"},
		},
		{
			name:   "only theirs changed",
			base:   &ParsedVersion{Body: "hello"},
			ours:   &ParsedVersion{Body: "hello"},
			theirs: &ParsedVersion{Body: "hello world"},
		},
		{
			name:   "both changed identically",
			base:   &ParsedVersion{Body: "hello"},
			ours:   &ParsedVersion{Body: "hello world"},
			theirs: &ParsedVersion{Body: "hello world"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ComputeBodyConflict(tt.base, tt.ours, tt.theirs)
			if result != nil {
				t.Errorf("expected nil body conflict, got %+v", result)
			}
		})
	}
}

func TestComputeBodyConflict_HasConflict(t *testing.T) {
	base := &ParsedVersion{Body: "line1\nline2\nline3"}
	ours := &ParsedVersion{Body: "line1\nline2 modified\nline3"}
	theirs := &ParsedVersion{Body: "line1\nline2\nline3 modified"}

	result := ComputeBodyConflict(base, ours, theirs)
	if result == nil {
		t.Fatal("expected body conflict, got nil")
	}

	if result.OurBody != ours.Body {
		t.Error("OurBody mismatch")
	}
	if result.TheirBody != theirs.Body {
		t.Error("TheirBody mismatch")
	}
	if len(result.Hunks) == 0 {
		t.Error("expected hunks, got none")
	}
}

func TestComputeBodyConflict_NonOverlapping(t *testing.T) {
	base := &ParsedVersion{Body: "line1\nline2\nline3\nline4\nline5"}
	ours := &ParsedVersion{Body: "line1\nours-new\nline2\nline3\nline4\nline5"}
	theirs := &ParsedVersion{Body: "line1\nline2\nline3\nline4\nline5\ntheirs-new"}

	result := ComputeBodyConflict(base, ours, theirs)
	if result == nil {
		t.Fatal("expected body conflict, got nil")
	}

	if !result.CanAutoMerge {
		t.Error("expected CanAutoMerge=true for non-overlapping changes")
	}
}

func TestDiffLines_Identical(t *testing.T) {
	a := []string{"a", "b", "c"}
	b := []string{"a", "b", "c"}
	ops := diffLines(a, b)

	for _, op := range ops {
		if op.Type != "equal" {
			t.Errorf("expected all equal ops, got %s", op.Type)
		}
	}
}

func TestDiffLines_Insert(t *testing.T) {
	a := []string{"a", "c"}
	b := []string{"a", "b", "c"}
	ops := diffLines(a, b)

	hasInsert := false
	for _, op := range ops {
		if op.Type == "insert" && op.Content == "b" {
			hasInsert = true
		}
	}
	if !hasInsert {
		t.Error("expected insert of 'b'")
	}
}

func TestDiffLines_Delete(t *testing.T) {
	a := []string{"a", "b", "c"}
	b := []string{"a", "c"}
	ops := diffLines(a, b)

	hasDelete := false
	for _, op := range ops {
		if op.Type == "delete" && op.Content == "b" {
			hasDelete = true
		}
	}
	if !hasDelete {
		t.Error("expected delete of 'b'")
	}
}

func TestDiffLines_Empty(t *testing.T) {
	ops := diffLines(nil, nil)
	if len(ops) != 0 {
		t.Errorf("expected 0 ops for empty inputs, got %d", len(ops))
	}
}

func TestSplitLines(t *testing.T) {
	tests := []struct {
		input string
		want  int
	}{
		{"", 0},
		{"a", 1},
		{"a\nb", 2},
		{"a\nb\nc", 3},
	}

	for _, tt := range tests {
		result := splitLines(tt.input)
		if len(result) != tt.want {
			t.Errorf("splitLines(%q): got %d lines, want %d", tt.input, len(result), tt.want)
		}
	}
}

func TestConflictFile_HasConflicts(t *testing.T) {
	cf := &ConflictFile{
		Fields: []FieldConflict{
			{Status: "unchanged"},
			{Status: "auto-ours"},
		},
	}
	if cf.HasConflicts() {
		t.Error("expected no conflicts")
	}

	cf.Fields = append(cf.Fields, FieldConflict{Status: "conflict"})
	if !cf.HasConflicts() {
		t.Error("expected conflicts")
	}
}

func TestConflictFile_CountConflictingFields(t *testing.T) {
	cf := &ConflictFile{
		Fields: []FieldConflict{
			{Status: "unchanged"},
			{Status: "conflict"},
			{Status: "auto-ours"},
			{Status: "conflict"},
		},
	}
	if got := cf.CountConflictingFields(); got != 2 {
		t.Errorf("got %d, want 2", got)
	}
}

func TestConflictSet_ConflictCount(t *testing.T) {
	cs := &ConflictSet{
		Files: []*ConflictFile{
			{Resolved: false},
			{Resolved: true},
			{Resolved: false},
		},
	}
	if got := cs.ConflictCount(); got != 2 {
		t.Errorf("got %d, want 2", got)
	}
}

func TestConflictSet_GetConflict(t *testing.T) {
	cs := &ConflictSet{
		Files: []*ConflictFile{
			{ID: "a"},
			{ID: "b"},
		},
	}

	if cf := cs.GetConflict("a"); cf == nil || cf.ID != "a" {
		t.Error("expected to find conflict 'a'")
	}
	if cf := cs.GetConflict("c"); cf != nil {
		t.Error("expected nil for non-existent conflict")
	}
}

func TestApplyResolution_Fields(t *testing.T) {
	cf := &ConflictFile{
		Fields: []FieldConflict{
			{Property: "title", Status: "unchanged", OurValue: "same", TheirValue: "same"},
			{Property: "status", Status: "auto-ours", OurValue: "open"},
			{Property: "priority", Status: "auto-theirs", TheirValue: "high"},
			{Property: "assignee", Status: "conflict", OurValue: "alice", TheirValue: "bob"},
		},
		Ours: &ParsedVersion{Body: "body text"},
	}

	resolved, body, err := cf.ApplyResolution(
		map[string]string{"assignee": "theirs"},
		nil,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resolved["title"] != "same" {
		t.Errorf("title: got %q, want 'same'", resolved["title"])
	}
	if resolved["status"] != "open" {
		t.Errorf("status: got %q, want 'open'", resolved["status"])
	}
	if resolved["priority"] != "high" {
		t.Errorf("priority: got %q, want 'high'", resolved["priority"])
	}
	if resolved["assignee"] != "bob" {
		t.Errorf("assignee: got %q, want 'bob'", resolved["assignee"])
	}
	if body != "body text" {
		t.Errorf("body: got %q, want 'body text'", body)
	}
	if !cf.Resolved {
		t.Error("expected Resolved=true")
	}
}

func TestApplyResolution_MissingChoice(t *testing.T) {
	cf := &ConflictFile{
		Fields: []FieldConflict{
			{Property: "status", Status: "conflict", OurValue: "a", TheirValue: "b"},
		},
		Ours: &ParsedVersion{Body: ""},
	}

	_, _, err := cf.ApplyResolution(map[string]string{}, nil)
	if err == nil {
		t.Error("expected error for missing field choice")
	}
}

func TestApplyResolution_BodyHunkChoice(t *testing.T) {
	cf := &ConflictFile{
		Fields: []FieldConflict{},
		Ours:   &ParsedVersion{Body: "our body"},
		BodyConflict: &BodyConflict{
			OurBody:   "our body",
			TheirBody: "their body",
			Hunks: []DiffHunk{
				{Index: 0, Source: "context", Lines: []DiffLine{{Type: "context", Content: "shared"}}},
				{Index: 1, Source: "conflict", Lines: []DiffLine{
					{Type: "add-ours", Content: "our line"},
					{Type: "add-theirs", Content: "their line"},
				}},
				{Index: 2, Source: "context", Lines: []DiffLine{{Type: "context", Content: "end"}}},
			},
		},
	}

	// Choose "theirs" for the conflict hunk
	_, body, err := cf.ApplyResolution(map[string]string{}, map[int]string{1: "theirs"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := "shared\ntheir line\nend"
	if body != expected {
		t.Errorf("got %q, want %q", body, expected)
	}

	// Reset and choose "ours"
	cf.Resolved = false
	_, body, err = cf.ApplyResolution(map[string]string{}, map[int]string{1: "ours"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected = "shared\nour line\nend"
	if body != expected {
		t.Errorf("got %q, want %q", body, expected)
	}
}

func TestApplyResolution_BodyHunkMissing(t *testing.T) {
	cf := &ConflictFile{
		Fields: []FieldConflict{},
		Ours:   &ParsedVersion{Body: "x"},
		BodyConflict: &BodyConflict{
			Hunks: []DiffHunk{
				{Index: 0, Source: "conflict", Lines: []DiffLine{
					{Type: "add-ours", Content: "a"},
					{Type: "add-theirs", Content: "b"},
				}},
			},
		},
	}
	// Missing hunk choice should error
	_, _, err := cf.ApplyResolution(map[string]string{}, map[int]string{})
	if err == nil {
		t.Error("expected error for missing hunk choice")
	}
}

func TestBuildTestConflictSet(t *testing.T) {
	cs := BuildTestConflictSet()

	if len(cs.Files) != 4 {
		t.Fatalf("expected 4 test conflicts, got %d", len(cs.Files))
	}

	// First conflict (TKT-004) should have field conflicts
	ticket := cs.GetConflict("TKT-004")
	if ticket == nil {
		t.Fatal("expected TKT-004 conflict")
	}
	if ticket.CountConflictingFields() == 0 {
		t.Error("TKT-004 should have conflicting fields")
	}
	if ticket.BodyConflict == nil {
		t.Error("TKT-004 should have body conflict")
	}

	// Second conflict (backend) should have field conflicts only
	cat := cs.GetConflict("backend")
	if cat == nil {
		t.Fatal("expected backend conflict")
	}
	if cat.CountConflictingFields() == 0 {
		t.Error("backend should have conflicting fields")
	}

	// Third conflict (TKT-010) has nil base
	newT := cs.GetConflict("TKT-010")
	if newT == nil {
		t.Fatal("expected TKT-010 conflict")
	}
	if newT.Base != nil {
		t.Error("TKT-010 should have nil base")
	}

	// Fourth conflict (DES-003) should have multiple body conflict hunks
	des := cs.GetConflict("DES-003")
	if des == nil {
		t.Fatal("expected DES-003 conflict")
	}
	if des.BodyConflict == nil {
		t.Fatal("DES-003 should have body conflict")
	}
	if got := des.BodyConflict.CountConflictHunks(); got < 2 {
		t.Errorf("DES-003: expected at least 2 conflict hunks, got %d", got)
	}
}

func TestHasOverlappingChanges(t *testing.T) {
	noOverlap := []DiffHunk{
		{Source: "ours"},
		{Source: "context"},
		{Source: "theirs"},
	}
	if hasOverlappingChanges(noOverlap) {
		t.Error("expected no overlapping changes")
	}

	withOverlap := []DiffHunk{
		{Source: "ours"},
		{Source: "conflict"},
	}
	if !hasOverlappingChanges(withOverlap) {
		t.Error("expected overlapping changes")
	}
}

func TestResolveBodyFromHunks_NoConflicts(t *testing.T) {
	bc := &BodyConflict{
		Hunks: []DiffHunk{
			{Index: 0, Source: "context", Lines: []DiffLine{{Type: "context", Content: "line1"}}},
			{Index: 1, Source: "ours", Lines: []DiffLine{{Type: "add-ours", Content: "new-ours"}}},
			{Index: 2, Source: "context", Lines: []DiffLine{{Type: "context", Content: "line2"}}},
			{Index: 3, Source: "theirs", Lines: []DiffLine{{Type: "add-theirs", Content: "new-theirs"}}},
			{Index: 4, Source: "context", Lines: []DiffLine{{Type: "context", Content: "line3"}}},
		},
		CanAutoMerge: true,
	}

	result, err := resolveBodyFromHunks(bc, map[int]string{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := "line1\nnew-ours\nline2\nnew-theirs\nline3"
	if result != expected {
		t.Errorf("got %q, want %q", result, expected)
	}
}

func TestResolveBodyFromHunks_WithConflicts(t *testing.T) {
	bc := &BodyConflict{
		Hunks: []DiffHunk{
			{Index: 0, Source: "context", Lines: []DiffLine{{Type: "context", Content: "start"}}},
			{Index: 1, Source: "conflict", Lines: []DiffLine{
				{Type: "add-ours", Content: "ours-a"},
				{Type: "add-ours", Content: "ours-b"},
				{Type: "add-theirs", Content: "theirs-a"},
			}},
			{Index: 2, Source: "context", Lines: []DiffLine{{Type: "context", Content: "end"}}},
		},
	}

	// Pick ours
	result, err := resolveBodyFromHunks(bc, map[int]string{1: "ours"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "start\nours-a\nours-b\nend" {
		t.Errorf("got %q", result)
	}

	// Pick theirs
	result, err = resolveBodyFromHunks(bc, map[int]string{1: "theirs"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "start\ntheirs-a\nend" {
		t.Errorf("got %q", result)
	}
}

func TestCountConflictHunks(t *testing.T) {
	bc := &BodyConflict{
		Hunks: []DiffHunk{
			{Source: "context"},
			{Source: "conflict"},
			{Source: "ours"},
			{Source: "conflict"},
		},
	}
	if got := bc.CountConflictHunks(); got != 2 {
		t.Errorf("got %d, want 2", got)
	}
}

// --- Phase 4 tests: git-integrated conflict resolution helpers ---

func TestEntityInfoFromPath(t *testing.T) {
	tests := []struct {
		path     string
		wantType string
		wantID   string
	}{
		{"entities/ticket/TKT-001.md", "ticket", "TKT-001"},
		{"entities/decision/DES-003.md", "decision", "DES-003"},
		{"relations/REQ-001--implements--SOL-001.md", "relations", "REQ-001--implements--SOL-001"},
		{"simple.md", ".", "simple"},
	}

	for _, tt := range tests {
		gotType, gotID := entityInfoFromPath(tt.path)
		if gotType != tt.wantType {
			t.Errorf("entityInfoFromPath(%q) type = %q, want %q", tt.path, gotType, tt.wantType)
		}
		if gotID != tt.wantID {
			t.Errorf("entityInfoFromPath(%q) id = %q, want %q", tt.path, gotID, tt.wantID)
		}
	}
}

func TestCollectPropertyNames(t *testing.T) {
	base := &ParsedVersion{Properties: map[string]string{"a": "1", "b": "2"}}
	ours := &ParsedVersion{Properties: map[string]string{"b": "2", "c": "3"}}
	theirs := &ParsedVersion{Properties: map[string]string{"c": "3", "d": "4"}}

	names := collectPropertyNames(base, ours, theirs)

	// All unique names should be present
	nameSet := make(map[string]bool)
	for _, n := range names {
		nameSet[n] = true
	}
	for _, want := range []string{"a", "b", "c", "d"} {
		if !nameSet[want] {
			t.Errorf("missing property %q", want)
		}
	}
}

func TestCollectPropertyNames_NilVersions(t *testing.T) {
	ours := &ParsedVersion{Properties: map[string]string{"x": "1"}}
	names := collectPropertyNames(nil, ours, nil)
	if len(names) != 1 || names[0] != "x" {
		t.Errorf("got %v, want [x]", names)
	}
}

func TestCapitalizeFirst(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"hello", "Hello"},
		{"", ""},
		{"due_date", "Due date"},
		{"estimated_hours", "Estimated hours"},
		{"a", "A"},
	}
	for _, tt := range tests {
		if got := capitalizeFirst(tt.input); got != tt.want {
			t.Errorf("capitalizeFirst(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestParseFileList(t *testing.T) {
	input := "entities/ticket/TKT-001.md\nentities/ticket/TKT-002.md\n\n"
	got := parseFileList(input)
	if len(got) != 2 {
		t.Fatalf("got %d files, want 2", len(got))
	}
	if got[0] != "entities/ticket/TKT-001.md" {
		t.Errorf("got[0] = %q", got[0])
	}
}

func TestParseFileList_Empty(t *testing.T) {
	got := parseFileList("")
	if len(got) != 0 {
		t.Errorf("got %d files for empty input, want 0", len(got))
	}
}

func TestParseVersionFromContent(t *testing.T) {
	content := "---\ntitle: Hello\nstatus: open\n---\nBody content here."
	pv := parseVersionFromContent(content)

	if pv.Properties["title"] != "Hello" {
		t.Errorf("title = %q, want Hello", pv.Properties["title"])
	}
	if pv.Properties["status"] != "open" {
		t.Errorf("status = %q, want open", pv.Properties["status"])
	}
	if !strings.Contains(pv.Body, "Body content here") {
		t.Errorf("body = %q, expected to contain 'Body content here'", pv.Body)
	}
}

func TestParseVersionFromContent_InvalidYAML(t *testing.T) {
	// No frontmatter — should fall back to body-only
	content := "Just plain text, no frontmatter."
	pv := parseVersionFromContent(content)

	// Should have an empty or non-nil properties map
	if pv.Properties == nil {
		t.Error("expected non-nil properties")
	}
}

func TestFormatResolvedDocument(t *testing.T) {
	props := map[string]string{"title": "Test", "status": "open"}
	body := "Some body content."

	result, err := FormatResolvedDocument(props, body)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "title: Test") {
		t.Errorf("result missing title: %s", result)
	}
	if !strings.Contains(result, "Some body content") {
		t.Errorf("result missing body: %s", result)
	}
}

func TestGitOutput(t *testing.T) {
	// Test gitOutput with a simple git command
	dir := t.TempDir()
	runGit(t, dir, "init")

	out, err := gitOutput(dir, "rev-parse", "--show-toplevel")
	if err != nil {
		t.Fatalf("gitOutput failed: %v", err)
	}
	if strings.TrimSpace(out) == "" {
		t.Error("expected non-empty output")
	}
}

func TestGitOutput_Error(t *testing.T) {
	dir := t.TempDir()
	// No git repo — should fail
	_, err := gitOutput(dir, "rev-parse", "--show-toplevel")
	if err == nil {
		t.Error("expected error for non-git directory")
	}
}

// TestBuildConflictSetFromGit creates a real git repo with diverging branches
// and verifies that BuildConflictSetFromGit correctly identifies conflicting files.
func TestBuildConflictSetFromGit(t *testing.T) {
	dir := t.TempDir()

	// Initialize repo with a known branch name
	runGit(t, dir, "init", "-b", "main")
	runGit(t, dir, "config", "user.email", "test@test.com")
	runGit(t, dir, "config", "user.name", "Test")

	// Create initial file on main
	entDir := filepath.Join(dir, "entities", "ticket")
	if err := os.MkdirAll(entDir, 0o755); err != nil {
		t.Fatal(err)
	}
	baseContent := "---\ntitle: Base Title\nstatus: open\n---\nBase body content."
	if err := os.WriteFile(filepath.Join(entDir, "TKT-001.md"), []byte(baseContent), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, dir, "add", "-A")
	runGit(t, dir, "commit", "-m", "initial")

	// Create a bare remote and push
	remoteDir := t.TempDir()
	runGit(t, remoteDir, "init", "--bare", "-b", "main")
	runGit(t, dir, "remote", "add", "origin", remoteDir)
	runGit(t, dir, "push", "-u", "origin", "main")

	// Simulate "theirs" changes on remote: clone, modify, push
	cloneParent := t.TempDir()
	cloneDir := filepath.Join(cloneParent, "work")
	runGit(t, cloneParent, "clone", remoteDir, "work")
	runGit(t, cloneDir, "config", "user.email", "them@test.com")
	runGit(t, cloneDir, "config", "user.name", "Them")
	theirsContent := "---\ntitle: Their Title\nstatus: resolved\n---\nTheir body content."
	if err := os.WriteFile(filepath.Join(cloneDir, "entities", "ticket", "TKT-001.md"), []byte(theirsContent), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, cloneDir, "add", "-A")
	runGit(t, cloneDir, "commit", "-m", "theirs")
	runGit(t, cloneDir, "push")

	// "Ours" changes: modify the same file differently (without fetching)
	oursContent := "---\ntitle: Our Title\nstatus: in-progress\n---\nOur body content."
	if err := os.WriteFile(filepath.Join(entDir, "TKT-001.md"), []byte(oursContent), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, dir, "add", "-A")
	runGit(t, dir, "commit", "-m", "ours")

	// Fetch to get origin/main
	runGit(t, dir, "fetch", "origin")

	// Now build conflicts
	cs, err := BuildConflictSetFromGit(dir, "main")
	if err != nil {
		t.Fatalf("BuildConflictSetFromGit failed: %v", err)
	}

	if len(cs.Files) == 0 {
		t.Fatal("expected at least 1 conflict file, got 0")
	}

	cf := cs.Files[0]
	if cf.EntityType != "ticket" {
		t.Errorf("entity type = %q, want ticket", cf.EntityType)
	}
	if cf.EntityID != "TKT-001" {
		t.Errorf("entity ID = %q, want TKT-001", cf.EntityID)
	}

	// Should have field conflicts (title and status changed on both sides)
	conflictCount := cf.CountConflictingFields()
	if conflictCount == 0 {
		t.Error("expected field conflicts")
	}

	// Verify the three versions are parsed correctly
	if cf.Base == nil {
		t.Error("expected non-nil base version")
	}
	if cf.Ours == nil {
		t.Fatal("expected non-nil ours version")
	}
	if cf.Theirs == nil {
		t.Fatal("expected non-nil theirs version")
	}
	if cf.Ours.Properties["title"] != "Our Title" {
		t.Errorf("ours title = %q, want 'Our Title'", cf.Ours.Properties["title"])
	}
	if cf.Theirs.Properties["title"] != "Their Title" {
		t.Errorf("theirs title = %q, want 'Their Title'", cf.Theirs.Properties["title"])
	}
}

// TestBuildConflictSetFromGit_NoConflicts tests when files changed on both sides
// but to the same values (no actual conflict).
func TestBuildConflictSetFromGit_NoConflicts(t *testing.T) {
	dir := t.TempDir()

	runGit(t, dir, "init", "-b", "main")
	runGit(t, dir, "config", "user.email", "test@test.com")
	runGit(t, dir, "config", "user.name", "Test")

	// Create initial file
	entDir := filepath.Join(dir, "entities", "ticket")
	if err := os.MkdirAll(entDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(entDir, "TKT-001.md"), []byte("---\ntitle: Same\n---\nBody"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, dir, "add", "-A")
	runGit(t, dir, "commit", "-m", "initial")

	// Create bare remote + push
	remoteDir := t.TempDir()
	runGit(t, remoteDir, "init", "--bare", "-b", "main")
	runGit(t, dir, "remote", "add", "origin", remoteDir)
	runGit(t, dir, "push", "-u", "origin", "main")

	// Both sides change to same value
	cloneParent := t.TempDir()
	cloneDir := filepath.Join(cloneParent, "work")
	runGit(t, cloneParent, "clone", remoteDir, "work")
	runGit(t, cloneDir, "config", "user.email", "them@test.com")
	runGit(t, cloneDir, "config", "user.name", "Them")
	sameContent := "---\ntitle: Updated\n---\nUpdated body"
	if err := os.WriteFile(filepath.Join(cloneDir, "entities", "ticket", "TKT-001.md"), []byte(sameContent), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, cloneDir, "add", "-A")
	runGit(t, cloneDir, "commit", "-m", "theirs")
	runGit(t, cloneDir, "push")

	if err := os.WriteFile(filepath.Join(entDir, "TKT-001.md"), []byte(sameContent), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, dir, "add", "-A")
	runGit(t, dir, "commit", "-m", "ours")
	runGit(t, dir, "fetch", "origin")

	cs, err := BuildConflictSetFromGit(dir, "main")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Both sides changed to same content — no actual conflicts
	if len(cs.Files) != 0 {
		t.Errorf("expected 0 conflict files (same changes), got %d", len(cs.Files))
	}
}

// runGit is defined in sync_test.go — reused here.
