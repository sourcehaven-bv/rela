---
id: REV-USS35G
type: review-checklist
title: 'Review: Lua code sharing between scripts (shared modules via require)'
started: "2026-10-14"
completed: "2026-10-14"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`): race tests pass for lua, script, mcp, validation, datamigration, dataentry, cli, scheduler; appbuild with `-tags sqlite`; the full suite runs in `just coverage-check`
- [x] Lint clean (`just lint`): golangci-lint 0 issues on the changed packages for both tag sets; plimsoll and arch-lint clean; markdownlint clean; eslint clean on the changed frontend files
- [x] Comment lint gate clean (`just comment-lint`)
- [x] Coverage maintained (`just coverage-check`): thresholds pass, total 83.8%. The only failing test, cmd/rela-desktop TestChromeStyle_TargetsShippedClasses, is macOS-only and needs the built SPA in static/; it is unrelated to this change and skipped on CI (Linux)

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent): cranky-code-reviewer and rela-security-reviewer both ran
- [x] All critical review-responses addressed (none raised)
- [x] All significant review-responses addressed (RR-F5G8ZD inline LuaPath, RR-2OT3VK thread cost and depth cap)
- [x] Self-reviewed the diff for unrelated changes (prettier reformatting of untouched lines reverted)

**Review Responses:** RR-729FF2 (security, minor), RR-F5G8ZD, RR-2OT3VK,
RR-IOSK6P, RR-COS0LR, RR-GLB5OO, RR-MCI945, RR-5G50JZ, RR-IL8W1J. All addressed.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

- AC1 PASS: TestRequire_ActionAndScheduledScriptShareModule; manual `rela script` run printed 2026-10-14.
- AC2 PASS: TestRequire_ModuleRunsOncePerRun.
- AC3 PASS: TestRequire_ModuleInheritsReaderRuntime.
- AC4 PASS: TestRequire_InvalidNames; manual `require("../etc/passwd")` rejected.
- AC5 PASS: TestRequire_Errors symlink case; sqlite subtest asserts no fallback to rela.db.
- AC6 PASS: TestRequire_Errors cycle and self-cycle.
- AC7 PASS: TestRequire_Errors missing module; TestRequire_UnreadableModule checks no absolute path.
- AC8 PASS: TestRequire_ErrorInModuleOnEverySurface (excerpt of lib/fail.lua:3).
- AC9 PASS: TestRequire_StandardLibraries, TestRequire_StandardLibraryIgnoresReassignedGlobal.
- AC10 PASS: TestSandbox_ModuleGlobalRemoved; luastep SandboxHasNoIO.
- AC11 PASS: dbonly_sqlite_test.go.
- AC12 PASS: TestRequire_CacheUsesEntryScriptNamespace.
- AC13 PASS: TestRequire_EntryScriptUnderLibSubdirKeepsItsFrames.
- AC14 PASS: assertModuleFailure (Error() prefix lib/fail.lua:3); ScriptErrorPanel.test.ts.
- AC15 PASS: TestRequire_CompileErrorPointsAtModuleLine, TestRequire_NestedCompileErrorPointsAtInnerModule, require_yield_test top-level frame.
- AC16 PASS: require_yield_test.go.
- AC17 PASS: lua_require_test.go.
- AC18 PASS: TestRequire_InvalidNames reserved and non-string cases.
- AC19 PASS: dbonly_sqlite_test.go (lib/README.md not stored).

## Documentation (enhancements only)

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated (GUIDE-lua-scripting, GUIDE-data-migration, GUIDE-sqlite-backend; docs/ regenerated)
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-MF4ZRD

## Final Checks

- [x] Commit message explains the why, not just what (drafted; commit waits for the user)
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] ~~Run `/pr` command to create PR and monitor CI~~ (N/A: deferred; the user decides when to commit and open a PR)
