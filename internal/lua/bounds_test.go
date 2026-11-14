package lua

import (
	"bytes"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/store"
)

func newBoundsRuntime(t *testing.T) *Runtime {
	t.Helper()
	var buf bytes.Buffer
	rt := NewReader(ReadDeps{World: store.TrivialScope()}, &buf)
	t.Cleanup(rt.Close)
	return rt
}

func TestLuaBounds_Refused(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		script  string
		wantErr string
	}{
		{"string.rep of 1 GiB", `local s = string.rep("x", 1073741824)`, "string.rep: result would exceed"},
		{"method rep of 1 GiB", `local s = ("x"):rep(1073741824)`, "string.rep: result would exceed"},
		{"string.rep overflow", `local s = string.rep("xy", 2^53)`, "string.rep: result would exceed"},
		{"string.rep just over", `local s = string.rep("x", 16 * 1048576 + 1)`, "string.rep: result would exceed"},
		{"format width", `local s = string.format("%999d", 1)`, "width or precision too long"},
		{"format precision", `local s = string.format("%.100f", 1)`, "width or precision too long"},
		{
			"format of many verbs",
			`local f = string.rep("%99d", 40000); local s = string.format(f, 1)`,
			"string.format: result would exceed",
		},
		{
			"format of large args",
			`local x = string.rep("x", 9 * 1048576); local s = string.format("%s%s", x, x)`,
			"string.format: result would exceed",
		},
		{
			"table.concat over the cap",
			`local x = string.rep("x", 1048576); local t = {}
			 for i = 1, 17 do t[i] = x end
			 local s = table.concat(t)`,
			"table.concat: result would exceed",
		},
		{
			"table.concat separator counts",
			`local t = {}; for i = 1, 20 do t[i] = "a" end
			 local s = table.concat(t, string.rep("-", 1048576))`,
			"table.concat: result would exceed",
		},
		{"deep recursion", `local function f(n) return 1 + f(n + 1) end; f(1)`, "stack overflow"},
		{
			"value stack past the registry maximum",
			`local t = {}; for i = 1, 70000 do t[i] = i end; local x = {unpack(t)}`,
			"registry overflow",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			rt := newBoundsRuntime(t)
			start := time.Now()
			err := rt.RunString(tc.script)
			if err == nil {
				t.Fatal("script succeeded")
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("error = %v, want it to contain %q", err, tc.wantErr)
			}
			if d := time.Since(start); d > 5*time.Second {
				t.Fatalf("refusal took %v", d)
			}
		})
	}
}

func TestLuaBounds_NormalScriptsPass(t *testing.T) {
	t.Parallel()
	rt := newBoundsRuntime(t)
	if err := rt.RunString(`
		assert(string.rep("ab", 3) == "ababab")
		assert(("-"):rep(4) == "----")
		assert(string.rep("x", 0) == "")
		assert(string.rep("x", -1) == "")
		assert(string.rep("", 1e9) == "")
		assert(#string.rep("x", 16 * 1048576) == 16 * 1048576)

		assert(string.format("%5.2f|%-3d|%s|%%|%02x", 3.14159, 7, "hi", 255) == " 3.14|7  |hi|%|ff")
		assert(string.format("%99d", 1):len() == 99)
		assert(string.format("no verbs") == "no verbs")

		assert(table.concat({"a", "b", "c"}, ",") == "a,b,c")
		assert(table.concat({1, 2, 3}) == "123")
		assert(table.concat({"a", "b", "c"}, "", 2) == "bc")
		assert(table.concat({"a", "b", "c"}, "-", 1, 2) == "a-b")
		assert(table.concat({}) == "")
		local ok = pcall(table.concat, {{}})
		assert(not ok, "concat of a table element must still raise")

		local function depth(n) if n == 0 then return 0 end return 1 + depth(n - 1) end
		assert(depth(500) == 500)

		local t = {}
		for i = 1, 10000 do t[i] = i end
		local packed = {unpack(t)}
		assert(#packed == 10000)
	`); err != nil {
		t.Fatalf("RunString: %v", err)
	}
}

func TestLuaBounds_SandboxHasCaps(t *testing.T) {
	t.Parallel()
	ls := NewSandbox(nil, nil, nil)
	defer ls.Close()
	err := ls.DoString(`local s = string.rep("x", 1073741824)`)
	if err == nil || !strings.Contains(err.Error(), "string.rep: result would exceed") {
		t.Fatalf("err = %v", err)
	}
}

// TestLuaBounds_RegistryOverflowDoesNotPanic pins the recover in
// pcallWithCapture: gopher-lua panics out of PCall when the value stack is
// full, and the runtime must turn that into an error and refuse later runs.
func TestLuaBounds_RegistryOverflowDoesNotPanic(t *testing.T) {
	t.Parallel()
	for _, script := range []string{
		`local t = {}; for i = 1, 70000 do t[i] = i end; local x = {unpack(t)}`,
		`local t = {}; for i = 1, 70000 do t[i] = i end
		 xpcall(function() return unpack(t) end, function(e) return e end)`,
	} {
		rt := newBoundsRuntime(t)
		if err := rt.RunString(script); err == nil {
			t.Fatal("overflowing script succeeded")
		}
		if err := rt.RunString(`return 1`); !errors.Is(err, errRuntimeBroken) {
			t.Fatalf("second run: err = %v, want errRuntimeBroken", err)
		}
	}
}
