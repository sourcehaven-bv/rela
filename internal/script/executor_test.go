package script

import (
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/lua"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
	"github.com/Sourcehaven-BV/rela/internal/tracer"
)

func testWriteDeps(projectRoot string) lua.WriteDeps {
	st := memstore.New()
	return lua.WriteDeps{
		ReadDeps: lua.ReadDeps{
			Store:       st,
			Tracer:      tracer.New(st),
			ProjectRoot: projectRoot,
		},
	}
}

func TestEngine_ExecuteFile_PathTraversal(t *testing.T) {
	engine := NewEngine()
	err := engine.ExecuteFile("../../../etc/passwd", testWriteDeps("/project"), "", nil, nil)
	if err == nil {
		t.Fatal("expected error for path traversal, got none")
	}
	if !strings.Contains(err.Error(), "local") {
		t.Errorf("expected path traversal error, got: %v", err)
	}
}

func TestEngine_ExecuteFile_AbsolutePath(t *testing.T) {
	engine := NewEngine()
	err := engine.ExecuteFile("/etc/passwd", testWriteDeps("/project"), "", nil, nil)
	if err == nil {
		t.Fatal("expected error for absolute path, got none")
	}
	if !strings.Contains(err.Error(), "local") {
		t.Errorf("expected path error, got: %v", err)
	}
}

func TestEngine_ExecuteFile_WrongExtension(t *testing.T) {
	engine := NewEngine()
	err := engine.ExecuteFile("script.txt", testWriteDeps("/project"), "", nil, nil)
	if err == nil {
		t.Fatal("expected error for wrong extension, got none")
	}
	if !strings.Contains(err.Error(), ".lua") {
		t.Errorf("expected extension error, got: %v", err)
	}
}

func TestEngine_ExecuteFile_ValidPath(t *testing.T) {
	engine := NewEngine()
	err := engine.ExecuteFile("test.lua", testWriteDeps("/nonexistent"), "", nil, nil)
	if err == nil {
		t.Fatal("expected error for missing project directory")
	}
	if strings.Contains(err.Error(), "local") || strings.Contains(err.Error(), ".lua") {
		t.Errorf("expected filesystem error, not validation error, got: %v", err)
	}
}
