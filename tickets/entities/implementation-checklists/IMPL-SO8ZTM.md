---
id: IMPL-SO8ZTM
type: implementation-checklist
title: 'Implementation: Lua code sharing between scripts (shared modules via require)'
started: "2026-10-14"
completed: "2026-10-14"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code (internal/lua/require_test.go, require_yield_test.go)
- [x] Integration tests written (internal/script/require_test.go covers action, automation file, automation inline and document; internal/validation/lua_require_test.go; mcp TestHandleLua_ErrorInModule; appbuild dbonly_sqlite_test.go with modules stored only in rela.db)
- [x] Happy path implemented
- [x] Edge cases from planning handled (invalid and reserved names, missing module, directory, cycle, self-cycle, compile and runtime errors, symlink escape, size cap, timeout, yield during load, detached runtime, failed load not cached)
- [x] Error handling in place (errors surfaced, not swallowed): every failure raises a Lua error naming the module and its project-relative path

## Test Quality

- [x] ~~Using fixture builders or factories for test data~~ (N/A: tests write small Lua files through a writeProject helper; no entity fixtures involved)
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] ~~Interpolated values constructed from objects, not hardcoded~~ (N/A: assertions compare against literal Lua source and paths the test itself wrote)
- [x] ~~Property comparisons use original object, not hardcoded strings~~ (N/A: no entity properties compared)

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

Built `.ignored/rela` from the tree and ran it on a fresh project (`rela init`)
with `lib/fmt/date.lua`:

- `rela script scripts/ok.lua` (`require("fmt.date").iso(2026,10,14)`) printed `2026-10-14`.
- `rela script scripts/bad.lua` (module raises on line 3) printed `lib/fmt/date.lua:3: bad date`.
- `rela script scripts/trav.lua` (`require("../etc/passwd")`) printed `scripts/trav.lua:1: bad argument #1 to require (invalid module name "../etc/passwd": ...)`.

The data-entry error panel shows the module file (ScriptErrorPanel.test.ts). The
sqlite db-only tier loads modules from rela.db (dbonly_sqlite_test.go). Race
tests pass for lua, script, mcp, validation, datamigration, dataentry, cli and
scheduler.

## Quality

- [x] Code follows project patterns (check similar code): reads go through ReadDeps.Files / rootfs like ReadScript; the error envelope reuses BuildScriptError
- [x] Checked for DRY opportunities: frame remapping removed from four call sites in favour of project-relative chunk names (RunChunk, projectPath)
- [x] No security issues introduced: allowlist name pattern, os.Root containment, package library never opened, migrations keep require nil
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
