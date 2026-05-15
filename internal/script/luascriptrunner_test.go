package script

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/autocascade"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/lua"
)

// recordingScriptExecutor lets tests observe which method
// luaScriptRunner picks (ExecuteCode vs ExecuteFile) and capture its
// arguments. Only the methods luaScriptRunner calls are programmed;
// LuaCache returns nil since these tests don't need cache wiring.
type recordingScriptExecutor struct {
	codeCalls []recordedExec
	fileCalls []recordedExec
	err       error
}

type recordedExec struct {
	code      string
	path      string
	newEntity *entity.Entity
	oldEntity *entity.Entity
}

func (r *recordingScriptExecutor) ExecuteCode(code string, _ lua.WriteDeps, newEntity, oldEntity *entity.Entity) error {
	r.codeCalls = append(r.codeCalls, recordedExec{code: code, newEntity: newEntity, oldEntity: oldEntity})
	return r.err
}

func (r *recordingScriptExecutor) ExecuteFile(path string, _ lua.WriteDeps, newEntity, oldEntity *entity.Entity) error {
	r.fileCalls = append(r.fileCalls, recordedExec{path: path, newEntity: newEntity, oldEntity: oldEntity})
	return r.err
}

// TestLuaScriptRunner_DispatchByActionShape — Code-only actions go to
// ExecuteCode; FilePath-only go to ExecuteFile; both-empty actions
// are a no-op.
func TestLuaScriptRunner_DispatchByActionShape(t *testing.T) {
	rec := &recordingScriptExecutor{}
	r := NewLuaScriptRunner(rec, lua.WriteDeps{})

	trigger := entity.New("REQ-001", "requirement")
	if err := r.Run(context.Background(), autocascade.ScriptAction{Code: "print('hi')", NewEntity: trigger}); err != nil {
		t.Fatalf("Run(Code): %v", err)
	}
	if err := r.Run(context.Background(), autocascade.ScriptAction{FilePath: "foo.lua", NewEntity: trigger}); err != nil {
		t.Fatalf("Run(FilePath): %v", err)
	}
	if err := r.Run(context.Background(), autocascade.ScriptAction{NewEntity: trigger}); err != nil {
		t.Fatalf("Run(empty): %v", err)
	}

	if len(rec.codeCalls) != 1 || rec.codeCalls[0].code != "print('hi')" {
		t.Errorf("expected one ExecuteCode call with the inline code, got %+v", rec.codeCalls)
	}
	if len(rec.fileCalls) != 1 || rec.fileCalls[0].path != "foo.lua" {
		t.Errorf("expected one ExecuteFile call with the path, got %+v", rec.fileCalls)
	}
}

// TestLuaScriptRunner_PatchesScriptErrorPath pins the
// engine-specific error-formatting behavior: when the executor
// returns *lua.ScriptError for an inline action (no FilePath), the
// adapter rewrites Path to "automation:<name>" so the error message
// identifies the failing block.
//
// This is the test that lived as autocascade.TestRunnerLuaErrorPath
// before the ScriptRunner extraction; it now lives at workspace
// because the patching is workspace-adapter behavior, not Runner
// behavior.
func TestLuaScriptRunner_PatchesScriptErrorPath(t *testing.T) {
	exec := &recordingScriptExecutor{
		err: &lua.ScriptError{
			Surface:    lua.SurfaceAutomation,
			Path:       "", // empty — simulates inline `lua: |` block
			LuaMessage: "boom",
		},
	}
	r := NewLuaScriptRunner(exec, lua.WriteDeps{})

	err := r.Run(context.Background(), autocascade.ScriptAction{
		Code: "error('boom')",
		Name: "my-automation",
	})
	if err == nil {
		t.Fatal("expected error from Run, got nil")
	}
	var se *lua.ScriptError
	if !errors.As(err, &se) {
		t.Fatalf("expected *lua.ScriptError, got %T", err)
	}
	if se.Path != "automation:my-automation" {
		t.Errorf("expected Path 'automation:my-automation', got %q", se.Path)
	}
	if !strings.Contains(err.Error(), "automation:my-automation") {
		t.Errorf("expected Error() to contain 'automation:my-automation', got %q", err.Error())
	}
}

// TestLuaScriptRunner_FilePathErrorNotPatched — when the action has
// a FilePath (a real script file, not inline), the adapter does NOT
// rewrite Path: the executor's path is already meaningful.
func TestLuaScriptRunner_FilePathErrorNotPatched(t *testing.T) {
	exec := &recordingScriptExecutor{
		err: &lua.ScriptError{
			Surface:    lua.SurfaceAutomation,
			Path:       "scripts/foo.lua",
			LuaMessage: "boom",
		},
	}
	r := NewLuaScriptRunner(exec, lua.WriteDeps{})

	err := r.Run(context.Background(), autocascade.ScriptAction{
		FilePath: "foo.lua",
		Name:     "my-automation",
	})
	if err == nil {
		t.Fatal("expected error from Run, got nil")
	}
	var se *lua.ScriptError
	if !errors.As(err, &se) {
		t.Fatalf("expected *lua.ScriptError, got %T", err)
	}
	if se.Path != "scripts/foo.lua" {
		t.Errorf("expected Path unchanged ('scripts/foo.lua'), got %q", se.Path)
	}
}

// TestLuaScriptRunner_NilOnNilExec — NewLuaScriptRunner returns nil
// when given a nil executor, so callers can pass the result to
// autocascade.Request.Scripts safely.
func TestLuaScriptRunner_NilOnNilExec(t *testing.T) {
	if got := NewLuaScriptRunner(nil, lua.WriteDeps{}); got != nil {
		t.Errorf("expected nil ScriptRunner for nil executor, got %#v", got)
	}
}
