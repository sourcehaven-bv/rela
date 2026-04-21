---
id: RR-4GQQ6
type: review-response
title: Cyclic Lua tables crash process with Go stack overflow
finding: 'validateRepresentable in internal/lua/cache.go recurses into *LTable via ForEach with no cycle detection. A two-line script `local t = {}; t.self = t; rela.cache.set(''k'', t)` recurses forever, hits Go''s 1GB stack limit, and terminates with `fatal error: stack overflow`. Not a panic that PCall catches — takes down the entire process. luaValueToGo in runtime.go has the same defect. Reproduced on current build.'
severity: critical
resolution: 'FULL FIX (round 2): pushed cycle detection down into luaValueToGo/luaTableToGoSeen in runtime.go via a seen map threaded through recursion. Every caller (rela.output, RunActionString, luaTableToGoMap, cache) now handles cyclic tables safely — cycles convert to the string ''<cyclic reference>'' instead of crashing the process. The round-1 fix at the cache boundary (validateRepresentable) is retained as a loud rejection; the round-2 fix in luaValueToGo makes the crash impossible regardless. New regression test: TestLuaValueToGoHandlesCycleWithoutCrash. E2E verified rela.output(cyclic) renders cleanly.'
status: addressed
---
