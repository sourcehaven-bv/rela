package script

import (
	"context"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/entitymanager"
	"github.com/Sourcehaven-BV/rela/internal/lua"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
	"github.com/Sourcehaven-BV/rela/internal/tracer"
)

// stubEntityManager is a no-op EntityManager for tests that exercise the
// script runtime wiring but never reach a mutation binding. It exists only
// to satisfy lua.NewWriter's construction-time non-nil check; every method
// panics so an accidental test reaching a mutation path fails loudly.
type stubEntityManager struct{}

func (stubEntityManager) CreateEntity(context.Context, *entity.Entity,
	entitymanager.CreateOptions) (*entitymanager.CreateResult, error) {
	panic("stubEntityManager.CreateEntity: not expected in this test")
}
func (stubEntityManager) UpdateEntity(context.Context,
	*entity.Entity) (*entitymanager.UpdateResult, error) {
	panic("stubEntityManager.UpdateEntity: not expected in this test")
}
func (stubEntityManager) DeleteEntity(context.Context, string,
	bool) (*entitymanager.DeleteResult, error) {
	panic("stubEntityManager.DeleteEntity: not expected in this test")
}
func (stubEntityManager) RenameEntity(context.Context, string, string,
	entitymanager.RenameOptions) (*entitymanager.RenameResult, error) {
	panic("stubEntityManager.RenameEntity: not expected in this test")
}
func (stubEntityManager) CreateRelation(context.Context, string, string, string,
	entitymanager.RelationOptions) (*entity.Relation, error) {
	panic("stubEntityManager.CreateRelation: not expected in this test")
}
func (stubEntityManager) UpdateRelation(context.Context, string, string, string,
	entitymanager.RelationOptions) (*entity.Relation, error) {
	panic("stubEntityManager.UpdateRelation: not expected in this test")
}
func (stubEntityManager) DeleteRelation(context.Context, string, string, string) error {
	panic("stubEntityManager.DeleteRelation: not expected in this test")
}

func testWriteDeps(projectRoot string) lua.WriteDeps {
	st := memstore.New()
	return lua.WriteDeps{
		ReadDeps: lua.ReadDeps{
			Store:       st,
			Tracer:      tracer.New(st),
			ProjectRoot: projectRoot,
		},
		EntityManager: stubEntityManager{},
	}
}

func TestEngine_ExecuteFile_PathTraversal(t *testing.T) {
	engine := NewEngine()
	err := engine.ExecuteFile("../../../etc/passwd", testWriteDeps("/project"), nil, nil)
	if err == nil {
		t.Fatal("expected error for path traversal, got none")
	}
	if !strings.Contains(err.Error(), "local") {
		t.Errorf("expected path traversal error, got: %v", err)
	}
}

func TestEngine_ExecuteFile_AbsolutePath(t *testing.T) {
	engine := NewEngine()
	err := engine.ExecuteFile("/etc/passwd", testWriteDeps("/project"), nil, nil)
	if err == nil {
		t.Fatal("expected error for absolute path, got none")
	}
	if !strings.Contains(err.Error(), "local") {
		t.Errorf("expected path error, got: %v", err)
	}
}

func TestEngine_ExecuteFile_WrongExtension(t *testing.T) {
	engine := NewEngine()
	err := engine.ExecuteFile("script.txt", testWriteDeps("/project"), nil, nil)
	if err == nil {
		t.Fatal("expected error for wrong extension, got none")
	}
	if !strings.Contains(err.Error(), ".lua") {
		t.Errorf("expected extension error, got: %v", err)
	}
}

func TestEngine_ExecuteFile_ValidPath(t *testing.T) {
	engine := NewEngine()
	err := engine.ExecuteFile("test.lua", testWriteDeps("/nonexistent"), nil, nil)
	if err == nil {
		t.Fatal("expected error for missing project directory")
	}
	if strings.Contains(err.Error(), "local") || strings.Contains(err.Error(), ".lua") {
		t.Errorf("expected filesystem error, not validation error, got: %v", err)
	}
}
