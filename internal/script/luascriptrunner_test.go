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

func (r *recordingScriptExecutor) ExecuteCode(_ context.Context, code string, _ lua.WriteDeps, newEntity, oldEntity *entity.Entity) error {
	r.codeCalls = append(r.codeCalls, recordedExec{code: code, newEntity: newEntity, oldEntity: oldEntity})
	return r.err
}

func (r *recordingScriptExecutor) ExecuteFile(_ context.Context, path string, _ lua.WriteDeps, newEntity, oldEntity *entity.Entity) error {
	r.fileCalls = append(r.fileCalls, recordedExec{path: path, newEntity: newEntity, oldEntity: oldEntity})
	return r.err
}

// stubMutator is type-correct but never invoked — every method panics
// so tests that accidentally call Mutator methods fail loudly.
// LuaScriptRunner only assigns it into lua.WriteDeps.EntityManager;
// the executor stubs above ignore that field, so no method actually
// fires in these tests.
type stubMutator struct{ autocascade.Mutator }

// TestLuaScriptRunner_DispatchByActionShape — Code-only actions go to
// ExecuteCode; FilePath-only go to ExecuteFile; both-empty actions
// are a no-op.
func TestLuaScriptRunner_DispatchByActionShape(t *testing.T) {
	rec := &recordingScriptExecutor{}
	r := NewLuaScriptRunner(rec, lua.ReadDeps{})

	trigger := entity.New("REQ-001", "requirement")
	if err := r.Run(context.Background(), autocascade.ScriptAction{Code: "print('hi')", NewEntity: trigger}, stubMutator{}); err != nil {
		t.Fatalf("Run(Code): %v", err)
	}
	if err := r.Run(context.Background(), autocascade.ScriptAction{FilePath: "foo.lua", NewEntity: trigger}, stubMutator{}); err != nil {
		t.Fatalf("Run(FilePath): %v", err)
	}
	if err := r.Run(context.Background(), autocascade.ScriptAction{NewEntity: trigger}, stubMutator{}); err != nil {
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
	r := NewLuaScriptRunner(exec, lua.ReadDeps{})

	err := r.Run(context.Background(), autocascade.ScriptAction{
		Code: "error('boom')",
		Name: "my-automation",
	}, stubMutator{})
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
	r := NewLuaScriptRunner(exec, lua.ReadDeps{})

	err := r.Run(context.Background(), autocascade.ScriptAction{
		FilePath: "foo.lua",
		Name:     "my-automation",
	}, stubMutator{})
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
	if got := NewLuaScriptRunner(nil, lua.ReadDeps{}); got != nil {
		t.Errorf("expected nil ScriptRunner for nil executor, got %#v", got)
	}
}

// TestLuaScriptRunner_RejectsNilMutator — Run with a non-empty action
// and nil mutator returns a typed error rather than letting the
// executor nil-deref on the first rela.create_entity call.
func TestLuaScriptRunner_RejectsNilMutator(t *testing.T) {
	r := NewLuaScriptRunner(&recordingScriptExecutor{}, lua.ReadDeps{})
	err := r.Run(context.Background(),
		autocascade.ScriptAction{Code: "print('hi')"}, nil)
	if err == nil {
		t.Fatal("expected error for nil mutator, got nil")
	}
	if !strings.Contains(err.Error(), "mutator is required") {
		t.Errorf("err = %v, want mutator-required message", err)
	}

	// Empty action with nil mutator is still a no-op (no executor
	// dispatch happens, so no mutator is needed).
	if err := r.Run(context.Background(),
		autocascade.ScriptAction{}, nil); err != nil {
		t.Errorf("empty action: expected no-op, got %v", err)
	}
}
