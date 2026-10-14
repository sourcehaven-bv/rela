package script

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/lua"
)

// failingModule raises on line 3 when its run() is called.
const failingModule = "local M = {}\nfunction M.run()\n  error(\"inside module\")\nend\nreturn M\n"

// writeProject writes files (project-relative path → content) under a temp
// project root and returns it.
func writeProject(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	for name, content := range files {
		p := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

// assertModuleFailure checks that err is a ScriptError that names the
// failing module line and carries its source excerpt, while Path stays the
// entry script.
func assertModuleFailure(t *testing.T, err error, wantPath string) *lua.ScriptError {
	t.Helper()
	var se *lua.ScriptError
	if !errors.As(err, &se) {
		t.Fatalf("expected *lua.ScriptError, got %T: %v", err, err)
	}
	if se.Path != wantPath {
		t.Errorf("Path = %q, want %q", se.Path, wantPath)
	}
	if se.LuaPath != "lib/fail.lua" || se.LuaLine != 3 {
		t.Errorf("LuaPath:LuaLine = %s:%d, want lib/fail.lua:3", se.LuaPath, se.LuaLine)
	}
	if !strings.HasPrefix(se.Error(), "lib/fail.lua:3: ") {
		t.Errorf("Error() = %q, want it to start with lib/fail.lua:3:", se.Error())
	}
	highlighted := ""
	for _, l := range se.Source {
		if l.Highlight {
			highlighted = l.Text
		}
	}
	if !strings.Contains(highlighted, `error("inside module")`) {
		t.Errorf("highlighted source = %q, want the module's failing line (source %+v)", highlighted, se.Source)
	}
	return se
}

func TestRequire_ActionAndScheduledScriptShareModule(t *testing.T) {
	t.Parallel()
	root := writeProject(t, map[string]string{
		"lib/fmt/date.lua": `return { iso = function(y, m, d) return string.format("%04d-%02d-%02d", y, m, d) end }`,
		"actions/a.lua":    `return { message = require("fmt.date").iso(2026, 1, 2) }`,
		"scripts/s.lua":    `assert(require("fmt.date").iso(2026, 3, 4) == "2026-03-04")`,
	})
	deps := testWriteDeps(root)
	engine := NewEngine()

	resp, err := engine.ExecuteAction(t.Context(), "a.lua", deps, nil, nil, 5*time.Second, "")
	if err != nil {
		t.Fatalf("action: %v", err)
	}
	if resp.Message != "2026-01-02" {
		t.Errorf("action message = %q", resp.Message)
	}
	if err := engine.ExecuteFile(t.Context(), "s.lua", deps, nil, nil); err != nil {
		t.Fatalf("scheduled script: %v", err)
	}
}

func TestRequire_ErrorInModuleOnEverySurface(t *testing.T) {
	t.Parallel()
	root := writeProject(t, map[string]string{
		"lib/fail.lua":    failingModule,
		"actions/a.lua":   "local m = require(\"fail\")\nm.run()\n",
		"scripts/s.lua":   "local m = require(\"fail\")\nm.run()\n",
		"scripts/doc.lua": "local m = require(\"fail\")\nm.run()\n",
	})
	deps := testWriteDeps(root)
	engine := NewEngine()

	t.Run("action", func(t *testing.T) {
		t.Parallel()
		_, err := engine.ExecuteAction(t.Context(), "a.lua", deps, nil, nil, 5*time.Second, "")
		se := assertModuleFailure(t, err, "actions/a.lua")
		if len(se.Stack) < 2 || se.Stack[1].Path != "actions/a.lua" || se.Stack[1].Line != 2 {
			t.Errorf("stack = %+v, want the caller frame actions/a.lua:2", se.Stack)
		}
	})
	t.Run("automation file", func(t *testing.T) {
		t.Parallel()
		err := engine.ExecuteFile(t.Context(), "s.lua", deps, nil, nil)
		se := assertModuleFailure(t, err, "scripts/s.lua")
		if len(se.Stack) < 2 || se.Stack[1].Path != "scripts/s.lua" {
			t.Errorf("stack = %+v, want the caller frame in scripts/s.lua", se.Stack)
		}
	})
	t.Run("automation inline", func(t *testing.T) {
		t.Parallel()
		err := engine.ExecuteCode(t.Context(), "require(\"fail\").run()", deps, nil, nil)
		assertModuleFailure(t, err, "<inline>")
	})
	t.Run("document", func(t *testing.T) {
		t.Parallel()
		var out bytes.Buffer
		err := engine.ExecuteDocument(t.Context(), "doc.lua", deps, &out, "d", "E-1", 0)
		assertModuleFailure(t, err, "scripts/doc.lua")
	})
}

// An entry script whose own path ends in lib/fail.lua must not have its
// frames confused with the module lib/fail.lua.
func TestRequire_EntryScriptUnderLibSubdirKeepsItsFrames(t *testing.T) {
	t.Parallel()
	root := writeProject(t, map[string]string{
		"lib/fail.lua":         failingModule,
		"actions/lib/fail.lua": "local m = require(\"fail\")\nm.run()\n",
	})
	_, err := NewEngine().ExecuteAction(t.Context(), "lib/fail.lua", testWriteDeps(root), nil, nil, 5*time.Second, "")
	se := assertModuleFailure(t, err, "actions/lib/fail.lua")
	if len(se.Stack) < 2 || se.Stack[0].Path != "lib/fail.lua" || se.Stack[1].Path != "actions/lib/fail.lua" {
		t.Errorf("stack = %+v, want lib/fail.lua then actions/lib/fail.lua", se.Stack)
	}
}
