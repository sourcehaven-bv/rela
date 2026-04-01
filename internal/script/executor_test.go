package script

import (
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/model"
)

// mockWorkspace implements lua.WorkspaceInterface for testing.
type mockWorkspace struct{}

func (m *mockWorkspace) GetEntity(_ string) (*model.Entity, bool) { return nil, false }
func (m *mockWorkspace) EntitiesByType(_ string) []*model.Entity  { return nil }
func (m *mockWorkspace) CreateEntityLua(_, _ string, _ map[string]interface{}, _ string) (*model.Entity, error) {
	return nil, nil //nolint:nilnil // test mock
}
func (m *mockWorkspace) UpdateEntityLua(_, _ *model.Entity) error { return nil }
func (m *mockWorkspace) DeleteEntityLua(_, _ string, _ bool) error {
	return nil
}
func (m *mockWorkspace) AllRelations() []*model.Relation { return nil }
func (m *mockWorkspace) CreateRelationLua(_, _, _ string) (*model.Relation, error) {
	return nil, nil //nolint:nilnil // test mock
}
func (m *mockWorkspace) DeleteRelation(_, _, _ string) error          { return nil }
func (m *mockWorkspace) TraceFrom(_ string, _ int) *model.TraceResult { return nil }
func (m *mockWorkspace) TraceTo(_ string, _ int) *model.TraceResult   { return nil }
func (m *mockWorkspace) FindPath(_, _ string) []model.PathStep        { return nil }
func (m *mockWorkspace) SearchSimple(_ string, _ int) ([]*model.Entity, error) {
	return nil, nil
}
func (m *mockWorkspace) SyncLua() error { return nil }

func TestExecutor_ExecuteFile_PathTraversal(t *testing.T) {
	// Test that path traversal attempts are blocked.
	exec := New(&mockWorkspace{}, nil, "/project")

	err := exec.ExecuteFile("../../../etc/passwd", nil, nil)
	if err == nil {
		t.Fatal("expected error for path traversal, got none")
	}
	if !strings.Contains(err.Error(), "local") {
		t.Errorf("expected path traversal error, got: %v", err)
	}
}

func TestExecutor_ExecuteFile_AbsolutePath(t *testing.T) {
	// Test that absolute paths are blocked.
	exec := New(&mockWorkspace{}, nil, "/project")

	err := exec.ExecuteFile("/etc/passwd", nil, nil)
	if err == nil {
		t.Fatal("expected error for absolute path, got none")
	}
	if !strings.Contains(err.Error(), "local") {
		t.Errorf("expected path error, got: %v", err)
	}
}

func TestExecutor_ExecuteFile_WrongExtension(t *testing.T) {
	// Test that non-.lua files are blocked.
	exec := New(&mockWorkspace{}, nil, "/project")

	err := exec.ExecuteFile("script.txt", nil, nil)
	if err == nil {
		t.Fatal("expected error for wrong extension, got none")
	}
	if !strings.Contains(err.Error(), ".lua") {
		t.Errorf("expected extension error, got: %v", err)
	}
}

func TestExecutor_ExecuteFile_ValidPath(t *testing.T) {
	// Test that valid paths pass validation (but fail at file access since
	// we don't have a real filesystem in tests).
	exec := New(&mockWorkspace{}, nil, "/nonexistent")

	err := exec.ExecuteFile("test.lua", nil, nil)
	// Should fail at project directory access (not validation)
	if err == nil {
		t.Fatal("expected error for missing project directory")
	}
	if strings.Contains(err.Error(), "local") || strings.Contains(err.Error(), ".lua") {
		t.Errorf("expected filesystem error, not validation error, got: %v", err)
	}
}
