package lua

import (
	"bytes"
	"errors"
	"strings"
	"testing"
	"time"

	glua "github.com/yuin/gopher-lua"

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
		{"format argument index", `local x = "x"; local s = string.format("%[1]s%[1]s", x)`, "not supported"},
		{
			"format argument index of a large arg",
			`local x = string.rep("x", 1048576); local s = string.format(string.rep("%[1]s", 100), x)`,
			"not supported",
		},
		{"format star width", `local s = string.format("%*d", 5, 1)`, "not supported"},
		{"format star precision", `local s = string.format("%.*f", 2, 1)`, "not supported"},
		{"format unknown verb", `local s = string.format("%v", 1)`, "invalid option '%v'"},
		{"format trailing percent", `local s = string.format("100%", 1)`, "invalid format"},
		{"format repeated flags", `local s = string.format("%-----d", 1)`, "repeated flags"},
		{"format missing argument", `local s = string.format("%s %s", "a")`, "no value"},
		{
			"format of many %s over one large arg each",
			`local x = string.rep("x", 1048576); local args = {}
			 for i = 1, 17 do args[i] = x end
			 local s = string.format(string.rep("%s", 17), unpack(args))`,
			"string.format: result would exceed",
		},
		{
			"format %q estimate",
			`local x = string.rep("\0", 5 * 1048576); local s = string.format("%q", x)`,
			"string.format: result would exceed",
		},
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

// TestBoundedStrFormat_MatchesGopherLua compares the bounded string.format
// with gopher-lua's own for valid formats. The output must not change.
func TestBoundedStrFormat_MatchesGopherLua(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		args string
	}{
		{"plain", `"no verbs"`},
		{"percent", `"100%%"`},
		{"percent between verbs", `"%d%%%s", 5, "x"`},
		{"string", `"%s", "hi"`},
		{"left aligned string", `"[%-10s]", "hi"`},
		{"right aligned string", `"[%10s]", "hi"`},
		{"string precision", `"%.2s", "hello"`},
		{"number as string", `"%s", 3.5`},
		{"integer as string", `"%s", 42`},
		{"quoted", `"%q", "a \"b\"\n\0"`},
		{"quoted number", `"%q", 7`},
		{"float", `"%5.2f", 3.14159`},
		{"float zero pad", `"%08.3f", -3.14159`},
		{"exp", `"%e|%E|%.3e", 12345.678, 0.00012, 1e300`},
		{"general", `"%g|%G|%g", 0.0001, 1e20, 100`},
		{"int", `"%d|%5d|%-5d|%05d|%+d", 7, 7, 7, 7, 7`},
		{"int from float", `"%d", 3.7`},
		{"int from string", `"%d", "12"`},
		{"i", `"%i", 9`},
		{"hex", `"%x|%X|%02x|%#x", 255, 255, 5, 255`},
		{"octal", `"%o|%#o", 8, 8`},
		{"char", `"%c%c", 72, 105`},
		{"space flag", `"% d", 5`},
		{"mixed", `"%5.2f|%-3d|%s|%%|%02x", 3.14159, 7, "hi", 255`},
		{"date", `"%04d-%02d-%02d", 2026, 1, 9`},
		{"extra args", `"%s", "a", "b"`},
		{"max width", `"%99d", 1`},
		{"bool as string", `"%s", true`},
		{"nil as string", `"%s", nil`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ref := glua.NewState()
			defer ref.Close()
			bounded := NewSandbox(nil, nil, nil)
			defer bounded.Close()
			src := "return string.format(" + tc.args + ")"
			if err := ref.DoString(src); err != nil {
				t.Fatalf("gopher-lua: %v", err)
			}
			want := ref.Get(-1).String()
			ref.Pop(1)
			if err := bounded.DoString(src); err != nil {
				t.Fatalf("bounded: %v", err)
			}
			got := bounded.Get(-1).String()
			bounded.Pop(1)
			if got != want {
				t.Fatalf("got %q, want %q", got, want)
			}
		})
	}
}

// TestBoundedStrFormat_ConvertsArguments pins the Lua 5.1 argument rules of
// string.format: a numeric verb takes a number, %s takes tostring of any
// value, and %q takes a string or a number. Go's fmt never sees a raw Lua
// value, so a table cannot print Go internals of unbounded size.
func TestBoundedStrFormat_ConvertsArguments(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		args string
		want string // the result, or "" when wantErr is set
		// wantErr is a substring of the argument error.
		wantErr string
	}{
		{name: "table as string", args: `"%s", {}`, want: "table: 0x"},
		{name: "function as string", args: `"%s", print`, want: "function: 0x"},
		{
			name: "__tostring applies to %s",
			args: `"[%s]", custom`,
			want: "[custom]",
		},
		{name: "byte above 127 is one byte", args: `"%c", 200`, want: "\xc8"},
		{name: "int from numeric string", args: `"%x", "255"`, want: "ff"},
		{name: "table to an integer verb", args: `"%d", {}`, wantErr: "number expected"},
		{name: "table to a float verb", args: `"%f", {}`, wantErr: "number expected"},
		{name: "table to %q", args: `"%q", {}`, wantErr: "string expected"},
		{name: "nil to %c", args: `"%c", nil`, wantErr: "number expected"},
		{name: "string to %x", args: `"%x", "abc"`, wantErr: "number expected"},
		{name: "big string to %x", args: `"%x", string.rep("a", 1048576)`, wantErr: "number expected"},
		{name: "big string to %X", args: `"%X", string.rep("a", 1048576)`, wantErr: "number expected"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ls := NewSandbox(nil, nil, nil)
			defer ls.Close()
			// Scripts cannot call setmetatable, so a value with __tostring
			// comes from Go, as a binding's would.
			custom, mt := ls.NewTable(), ls.NewTable()
			mt.RawSetString("__tostring", ls.NewFunction(func(l *glua.LState) int {
				l.Push(glua.LString("custom"))
				return 1
			}))
			ls.SetMetatable(custom, mt)
			ls.SetGlobal("custom", custom)
			err := ls.DoString("return string.format(" + tc.args + ")")
			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("error = %v, want an argument error containing %q", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			got := ls.Get(-1).String()
			if tc.want == "table: 0x" || tc.want == "function: 0x" {
				if !strings.HasPrefix(got, tc.want) || len(got) > 64 {
					t.Fatalf("got %q, want a short %q... address", got, tc.want)
				}
				return
			}
			if got != tc.want {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
}
