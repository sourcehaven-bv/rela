package lua

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// newModuleProject writes files (project-relative path → content) into a temp
// project directory and returns its root.
func newModuleProject(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	for name, content := range files {
		p := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func newModuleWriter(t *testing.T, root string, opts ...Option) *Runtime {
	t.Helper()
	ws := newMockWorkspace(t)
	r := NewWriter(ws.services(root), &bytes.Buffer{}, opts...)
	t.Cleanup(r.Close)
	return r
}

func TestRequire_LoadsModuleFromLib(t *testing.T) {
	t.Parallel()
	root := newModuleProject(t, map[string]string{
		"lib/fmt/date.lua": `local M = {}
function M.iso(y, m, d) return string.format("%04d-%02d-%02d", y, m, d) end
return M`,
		"lib/util.lua": `return { answer = 42 }`,
	})
	r := newModuleWriter(t, root)
	mustRun(t, r, `
local date = require("fmt.date")
assert(date.iso(2026, 1, 2) == "2026-01-02", "fmt.date")
assert(require("util").answer == 42, "util")
`)
}

func TestRequire_ModuleRunsOncePerRun(t *testing.T) {
	t.Parallel()
	root := newModuleProject(t, map[string]string{
		"lib/counter.lua": `loads = (loads or 0) + 1
return { id = {} }`,
	})
	r := newModuleWriter(t, root)
	mustRun(t, r, `
local a = require("counter")
local b = require("counter")
assert(loads == 1, "module ran " .. tostring(loads) .. " times")
assert(a == b, "require returned different values")
`)
}

func TestRequire_NilReturnBecomesTrue(t *testing.T) {
	t.Parallel()
	root := newModuleProject(t, map[string]string{
		"lib/sideeffect.lua": `marker = "set"`,
	})
	r := newModuleWriter(t, root)
	mustRun(t, r, `
assert(require("sideeffect") == true)
assert(marker == "set")
`)
}

func TestRequire_ModuleReceivesName(t *testing.T) {
	t.Parallel()
	root := newModuleProject(t, map[string]string{
		"lib/a/b.lua": `local name = ... ; return name`,
	})
	r := newModuleWriter(t, root)
	mustRun(t, r, `assert(require("a.b") == "a.b")`)
}

func TestRequire_StandardLibraries(t *testing.T) {
	t.Parallel()
	// A lib/string.lua must never shadow the standard library.
	root := newModuleProject(t, map[string]string{
		"lib/string.lua": `error("lib/string.lua must not load")`,
	})
	r := newModuleWriter(t, root)
	mustRun(t, r, `
assert(require("string") == string)
assert(require("table") == table)
assert(require("math") == math)
assert(require("coroutine") == coroutine)
`)
}

func TestRequire_InvalidNames(t *testing.T) {
	t.Parallel()
	root := newModuleProject(t, map[string]string{"lib/ok.lua": `return 1`})
	tests := []struct {
		name string
		arg  string
	}{
		{"empty", `""`},
		{"parent", `"../x"`},
		{"absolute", `"/etc/passwd"`},
		{"slash", `"a/b"`},
		{"backslash", `"a\\b"`},
		{"double dot", `"a..b"`},
		{"leading dot", `".a"`},
		{"trailing dot", `"a."`},
		{"upper case", `"Fmt"`},
		{"hyphen", `"a-b"`},
		{"extension", `"ok.lua"`},
		{"nul byte", `"ok\0"`},
		{"unicode", `"café"`},
		{"too long", `string.rep("a", 129)`},
		{"table", `{}`},
		{"nil", `nil`},
		{"number", `1`},
		{"reserved rela", `"rela"`},
		{"reserved http prefix", `"http.client"`},
		{"reserved os", `"os"`},
		{"stdlib submodule", `"string.extra"`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			r := newModuleWriter(t, root)
			err := mustRunErr(t, r, `require(`+tc.arg+`)`)
			if strings.Contains(err.Error(), root) {
				t.Errorf("error leaks the project path: %v", err)
			}
		})
	}
}

func TestRequire_Errors(t *testing.T) {
	t.Parallel()
	root := newModuleProject(t, map[string]string{
		"lib/cycle_a.lua":  `return require("cycle_b")`,
		"lib/cycle_b.lua":  `return require("cycle_a")`,
		"lib/self.lua":     `return require("self")`,
		"lib/broken.lua":   `return {`,
		"lib/raises.lua":   `error("boom")`,
		"secret.lua":       `return "secret"`,
		"lib/sub/keep.lua": `return 1`,
	})
	if err := os.Symlink(filepath.Join(root, "secret.lua"), filepath.Join(root, "lib", "escape.lua")); err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name string
		code string
		want string
	}{
		{"missing", `require("fmt.date")`, "module not found: fmt.date (looked in lib/fmt/date.lua)"},
		{"directory", `require("sub")`, "sub"},
		{"cycle", `require("cycle_a")`, "require cycle: cycle_a -> cycle_b -> cycle_a"},
		{"self cycle", `require("self")`, "require cycle: self -> self"},
		{"compile error", `require("broken")`, "cannot compile module broken"},
		{"runtime error", `require("raises")`, "boom"},
		{"symlink escape", `require("escape")`, "escape"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			r := newModuleWriter(t, root)
			err := mustRunErr(t, r, tc.code)
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error = %v, want it to contain %q", err, tc.want)
			}
			if strings.Contains(err.Error(), root) {
				t.Errorf("error leaks the project path: %v", err)
			}
			if strings.Contains(err.Error(), "secret") {
				t.Errorf("symlink escape was read: %v", err)
			}
		})
	}
}

func TestRequire_FailedLoadIsNotCached(t *testing.T) {
	t.Parallel()
	root := newModuleProject(t, map[string]string{
		"lib/flaky.lua": `attempts = (attempts or 0) + 1
error("fail")`,
	})
	r := newModuleWriter(t, root)
	mustRun(t, r, `
assert(not pcall(require, "flaky"))
assert(not pcall(require, "flaky"))
assert(attempts == 2, "attempts = " .. tostring(attempts))
`)
}

func TestRequire_ErrorFramesNameModule(t *testing.T) {
	t.Parallel()
	root := newModuleProject(t, map[string]string{
		"lib/fail.lua": "local M = {}\nfunction M.run()\n  error(\"inside module\")\nend\nreturn M",
	})
	r := newModuleWriter(t, root)
	_ = mustRunErr(t, r, `require("fail").run()`)
	frames := r.ErrorFrames()
	if len(frames) == 0 || frames[0].Path != "lib/fail.lua" || frames[0].Line != 3 {
		t.Fatalf("frames = %+v, want innermost lib/fail.lua:3", frames)
	}
}

func TestRequire_ModuleInheritsReaderRuntime(t *testing.T) {
	t.Parallel()
	root := newModuleProject(t, map[string]string{
		"lib/writes.lua": `return function() return rela.create_entity end`,
		"lib/net.lua":    `return function() return http end`,
	})
	ws := newMockWorkspace(t)
	r := NewReader(ws.readDeps(root), &bytes.Buffer{})
	t.Cleanup(r.Close)
	mustRun(t, r, `
assert(require("writes")() == nil, "reader runtime exposes create_entity to a module")
assert(require("net")() == nil, "module sees http without the capability")
`)
}

func TestRequire_DetachedRuntimeHasNoModules(t *testing.T) {
	t.Parallel()
	r := NewDetached(&bytes.Buffer{})
	t.Cleanup(r.Close)
	err := mustRunErr(t, r, `require("fmt.date")`)
	if !strings.Contains(err.Error(), "not available in this context") {
		t.Errorf("error = %v", err)
	}
	mustRun(t, r, `assert(require("string") == string)`)
}

func TestRequire_CacheUsesEntryScriptNamespace(t *testing.T) {
	t.Parallel()
	root := newModuleProject(t, map[string]string{
		"lib/memo.lua": `return { put = function(v) rela.cache.set("k", v) end }`,
	})
	ws := newMockWorkspace(t)
	c := NewCache()

	a := NewWriter(ws.services(root), &bytes.Buffer{}, WithCache(c))
	t.Cleanup(a.Close)
	a.SetScriptPath("scripts/a.lua")
	mustRun(t, a, `require("memo").put(7); assert(rela.cache.get("k") == 7)`)

	b := NewWriter(ws.services(root), &bytes.Buffer{}, WithCache(c))
	t.Cleanup(b.Close)
	b.SetScriptPath("scripts/b.lua")
	mustRun(t, b, `assert(rela.cache.get("k") == nil, "cache entry crossed script namespaces")`)
}

func TestSandbox_ModuleGlobalRemoved(t *testing.T) {
	t.Parallel()
	r := newModuleWriter(t, t.TempDir())
	mustRun(t, r, `assert(module == nil)`)
}

func TestRequire_StandardLibraryIgnoresReassignedGlobal(t *testing.T) {
	t.Parallel()
	r := newModuleWriter(t, t.TempDir())
	mustRun(t, r, `
local s = string
string = nil
assert(require("string") == s)
`)
}

func TestRequire_CompileErrorPointsAtModuleLine(t *testing.T) {
	t.Parallel()
	root := newModuleProject(t, map[string]string{
		"lib/broken.lua": "local M = {}\n\nfunction M.x(\nreturn M",
	})
	r := newModuleWriter(t, root)
	_ = mustRunErr(t, r, `local ok = 1
require("broken")`)
	frames := r.ErrorFrames()
	if len(frames) == 0 || frames[0].Path != "lib/broken.lua" || frames[0].Line < 3 {
		t.Fatalf("frames = %+v, want innermost in lib/broken.lua", frames)
	}
}

func TestRequire_CaughtCompileErrorDoesNotLeakIntoLaterError(t *testing.T) {
	t.Parallel()
	root := newModuleProject(t, map[string]string{"lib/broken.lua": "return {"})
	r := newModuleWriter(t, root)
	_ = mustRunErr(t, r, `pcall(require, "broken")
error("later")`)
	for _, f := range r.ErrorFrames() {
		if f.Path == "lib/broken.lua" {
			t.Fatalf("stale compile frame on a later error: %+v", r.ErrorFrames())
		}
	}
}

func TestRequire_ModuleTooLarge(t *testing.T) {
	t.Parallel()
	root := newModuleProject(t, map[string]string{
		"lib/big.lua": "return 1 --" + strings.Repeat("x", maxModuleSize),
	})
	r := newModuleWriter(t, root)
	err := mustRunErr(t, r, `require("big")`)
	if !strings.Contains(err.Error(), "larger than") {
		t.Errorf("error = %v", err)
	}
}

func TestRequire_TimeoutDuringModuleLoad(t *testing.T) {
	t.Parallel()
	root := newModuleProject(t, map[string]string{"lib/spin.lua": `while true do end`})
	r := newModuleWriter(t, root, WithTimeout(100*time.Millisecond))
	_ = mustRunErr(t, r, `require("spin")`)
}

func TestRequire_DeepChainIsCapped(t *testing.T) {
	t.Parallel()
	files := map[string]string{}
	const depth = 400
	for i := range depth {
		files[fmt.Sprintf("lib/m%d.lua", i)] = fmt.Sprintf(`return require("m%d")`, i+1)
	}
	files[fmt.Sprintf("lib/m%d.lua", depth)] = `return "bottom"`
	root := newModuleProject(t, files)
	r := newModuleWriter(t, root)
	err := mustRunErr(t, r, `require("m0")`)
	if !strings.Contains(err.Error(), fmt.Sprintf("nested more than %d deep", maxModuleDepth)) {
		t.Errorf("error = %v", err)
	}
}

func TestRequire_ChainAtDepthLimitLoads(t *testing.T) {
	t.Parallel()
	files := map[string]string{}
	for i := range maxModuleDepth - 1 {
		files[fmt.Sprintf("lib/m%d.lua", i)] = fmt.Sprintf(`return require("m%d")`, i+1)
	}
	files[fmt.Sprintf("lib/m%d.lua", maxModuleDepth-1)] = `return "bottom"`
	r := newModuleWriter(t, newModuleProject(t, files))
	mustRun(t, r, `assert(require("m0") == "bottom")`)
}

// A module's thread starts with a small registry and call stack; both must
// grow to the runtime's own limits.
func TestRequire_ModuleThreadGrowsToRuntimeLimits(t *testing.T) {
	t.Parallel()
	root := newModuleProject(t, map[string]string{
		"lib/heavy.lua": `
local function depth(n) if n == 0 then return 0 end return 1 + depth(n - 1) end
local t = {}
for i = 1, 5000 do t[i] = i end
return { depth = depth(900), n = #t, packed = select("#", unpack(t, 1, 4000)) }
`,
	})
	r := newModuleWriter(t, root)
	mustRun(t, r, `
local h = require("heavy")
assert(h.depth == 900 and h.n == 5000 and h.packed == 4000)
`)
}

func TestRequire_NonStringErrorPassesThrough(t *testing.T) {
	t.Parallel()
	root := newModuleProject(t, map[string]string{"lib/te.lua": `error({code = "E1"})`})
	r := newModuleWriter(t, root)
	mustRun(t, r, `
local ok, e = pcall(require, "te")
assert(not ok and type(e) == "table" and e.code == "E1", "error value = " .. tostring(e))
`)
}

func TestRequire_NestedCompileErrorPointsAtInnerModule(t *testing.T) {
	t.Parallel()
	root := newModuleProject(t, map[string]string{
		"lib/outer.lua": "local x = 1\nlocal i = require('inner')\nreturn i",
		"lib/inner.lua": "local M = {}\n\nfunction M.x(\nreturn M",
	})
	r := newModuleWriter(t, root)
	_ = mustRunErr(t, r, `require("outer")`)
	frames := r.ErrorFrames()
	if len(frames) == 0 || frames[0].Path != "lib/inner.lua" || frames[0].Line < 3 {
		t.Fatalf("frames = %+v, want innermost in lib/inner.lua", frames)
	}
}

func TestRequire_UnreadableModule(t *testing.T) {
	t.Parallel()
	if os.Geteuid() == 0 {
		t.Skip("root reads a mode-000 file")
	}
	root := newModuleProject(t, map[string]string{"lib/locked.lua": `return 1`})
	if err := os.Chmod(filepath.Join(root, "lib", "locked.lua"), 0); err != nil {
		t.Fatal(err)
	}
	r := newModuleWriter(t, root)
	err := mustRunErr(t, r, `require("locked")`)
	if !strings.Contains(err.Error(), "permission denied") || strings.Contains(err.Error(), root) {
		t.Errorf("error = %v, want permission denied without the absolute path", err)
	}
}
