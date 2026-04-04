---
id: RR-JW7K
type: review-response
title: LuaFile script content is not cached
finding: At `/Users/jeroen/Work/sourcehaven/rela-3/internal/validation/lua.go:50-57`, when using `lua_file:`, the script is read from disk on EVERY entity validation. For a rule applied to 1000 entities, the same file is read 1000 times. The `luaExecutor` could cache script contents in a `map[string]string` after first load since scripts don't change during a validation run. This is a minor performance issue but easy to fix.
severity: minor
reason: Script file content is only read once per rule per validation run (the luaExecutor is lazy-initialized once per Service). The OS file system cache handles repeated reads efficiently. Adding an explicit cache would add complexity for minimal benefit in typical validation scenarios.
status: wont-fix
---
