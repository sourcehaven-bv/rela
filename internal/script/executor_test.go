package script

import (
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/lua"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
	"github.com/Sourcehaven-BV/rela/internal/store/storetrace"
)

// testContext implements script.Context for testing.
type testContext struct {
	projectRoot string
}

func (c *testContext) GetWorkspace() interface{} {
	st := memstore.New()
	return lua.Services{
		Store:       st,
		Tracer:      storetrace.New(st),
		ProjectRoot: c.projectRoot,
	}
}
func (c *testContext) GetMeta() *metamodel.Metamodel { return nil }
func (c *testContext) GetProjectRoot() string        { return c.projectRoot }
func (c *testContext) GetEntity() *entity.Entity     { return nil }
func (c *testContext) GetOldEntity() *entity.Entity  { return nil }

func TestEngine_ExecuteFile_PathTraversal(t *testing.T) {
	// Test that path traversal attempts are blocked.
	engine := NewEngine()
	ctx := &testContext{projectRoot: "/project"}

	err := engine.ExecuteFile("../../../etc/passwd", ctx)
	if err == nil {
		t.Fatal("expected error for path traversal, got none")
	}
	if !strings.Contains(err.Error(), "local") {
		t.Errorf("expected path traversal error, got: %v", err)
	}
}

func TestEngine_ExecuteFile_AbsolutePath(t *testing.T) {
	// Test that absolute paths are blocked.
	engine := NewEngine()
	ctx := &testContext{projectRoot: "/project"}

	err := engine.ExecuteFile("/etc/passwd", ctx)
	if err == nil {
		t.Fatal("expected error for absolute path, got none")
	}
	if !strings.Contains(err.Error(), "local") {
		t.Errorf("expected path error, got: %v", err)
	}
}

func TestEngine_ExecuteFile_WrongExtension(t *testing.T) {
	// Test that non-.lua files are blocked.
	engine := NewEngine()
	ctx := &testContext{projectRoot: "/project"}

	err := engine.ExecuteFile("script.txt", ctx)
	if err == nil {
		t.Fatal("expected error for wrong extension, got none")
	}
	if !strings.Contains(err.Error(), ".lua") {
		t.Errorf("expected extension error, got: %v", err)
	}
}

func TestEngine_ExecuteFile_ValidPath(t *testing.T) {
	// Test that valid paths pass validation (but fail at file access since
	// we don't have a real filesystem in tests).
	engine := NewEngine()
	ctx := &testContext{projectRoot: "/nonexistent"}

	err := engine.ExecuteFile("test.lua", ctx)
	// Should fail at project directory access (not validation)
	if err == nil {
		t.Fatal("expected error for missing project directory")
	}
	if strings.Contains(err.Error(), "local") || strings.Contains(err.Error(), ".lua") {
		t.Errorf("expected filesystem error, not validation error, got: %v", err)
	}
}
