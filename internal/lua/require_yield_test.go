package lua

import (
	"strings"
	"testing"
)

func TestRequire_YieldDuringModuleLoadIsAnError(t *testing.T) {
	t.Parallel()
	root := newModuleProject(t, map[string]string{
		"lib/y.lua":     `coroutine.yield(1); return {}`,
		"lib/inner.lua": `local co = coroutine.wrap(function() coroutine.yield(5) end); return co()`,
		"lib/later.lua": `return { step = function() local v = coroutine.yield(2); return v end }`,
	})
	r := newModuleWriter(t, root)
	mustRun(t, r, `
local co = coroutine.create(function() return require("y") end)
local ok, err = coroutine.resume(co)
assert(not ok, "yield at module top level must fail")
assert(string.find(err, "yielded while loading", 1, true), err)

-- A coroutine the module creates and finishes itself is fine.
assert(require("inner") == 5)

-- Yielding from a function the module returns is fine.
local later = require("later")
local c2 = coroutine.create(function() return later.step() end)
local ok1, v1 = coroutine.resume(c2)
assert(ok1 and v1 == 2)
local ok2, v2 = coroutine.resume(c2, "resumed")
assert(ok2 and v2 == "resumed")
`)
}

func TestRequire_TopLevelErrorFrameNamesModule(t *testing.T) {
	t.Parallel()
	root := newModuleProject(t, map[string]string{"lib/raises.lua": "local x = 1\nerror(\"boom\")"})
	r := newModuleWriter(t, root)
	err := mustRunErr(t, r, `require("raises")`)
	if !strings.Contains(err.Error(), "boom") {
		t.Fatalf("err = %v", err)
	}
	frames := r.ErrorFrames()
	if len(frames) == 0 || frames[0].Path != "lib/raises.lua" || frames[0].Line != 2 {
		t.Fatalf("frames = %+v, want innermost lib/raises.lua:2", frames)
	}
}
