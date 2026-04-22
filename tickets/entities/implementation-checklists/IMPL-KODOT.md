---
id: IMPL-KODOT
type: implementation-checklist
title: 'Implementation: Document the documents feature and add Lua script renderer'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code
- [x] Integration tests written (test full flow, not just units)
- [x] Happy path implemented
- [x] Edge cases from planning handled
- [x] Error handling in place (errors surfaced, not swallowed)

## Test Quality

- [x] Using fixture builders or factories for test data
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

Covered by unit + integration tests; `just ci` (excluding docs-check which needs
the commit) passes clean.

Per-AC status:

| AC | Evidence |
|----|----------|
| AC1 Lua happy path | `TestDocumentService_ScriptRender_CapturesMarkdown` — passing |
| AC2 Config validation | `TestValidateConfig_Documents` (6 subtests: both, neither, only-command, only-script, missing entity_type) — passing |
| AC3 Context injection | `TestDocumentMode_ContextInjection` — asserts `rela.mode`, `rela.document.id`, `rela.document.entry_id` via print readback — passing |
| AC4 Context absent elsewhere | `TestDocumentMode_AbsentInOtherContexts` — vanilla writer runtime: both nil — passing |
| AC5 `rela.output` warning | `TestDocumentMode_OutputIsWarning` — stdout contains "warning" and "document mode", no JSON — passing |
| AC6 Cache memoize across renders | `TestDocumentService_CacheMemoizeAcrossRenders` — uses real `script.Engine`, counter.log has exactly 1 line after 2 renders — passing |
| AC7 Shell command unchanged | Existing `TestDocumentDiskCache` and friends still pass; `TestHandleV1Documents_EntityTypeMatch` exercises the command: path end-to-end — passing |
| AC8 Singleflight keyed on configID | `TestDocumentService_SingleflightNoCollapseAcrossConfigs` + positive complement `TestDocumentService_SingleflightCollapsesSameConfig` — passing |
| AC9 EntityType enforcement | `TestHandleV1Documents_EntityTypeMismatch` (400), `TestHandleV1Documents_EntityTypeMatch` (no-400), `TestHandleV1Documents_EntityNotFound` (404) — passing |
| AC10 Disk cache bypass for script | `TestDocumentService_ScriptRender_NoDiskCacheWrite` (no write) + `TestDocumentService_ScriptRender_StaleCommandCacheIgnored` (stale not served) — passing |
| AC11 `cfg.Timeout` honored | `TestExecuteDocument_TimeoutEnforced` — infinite loop terminates within ~1s when `timeout: 1` is set — passing |
| AC-DOC1 Guide section | `docs-project/entities/guides/GUIDE-data-entry.md` has a new `## Documents` section (YAML schema, URL schemes, caching, SSE caveat, security, hot-reload caveat) — docs regenerated into `docs/data-entry.md` |
| AC-DOC2 FEAT-023 updated | Content and status (implemented) updated |
| AC-DOC3 Prototype example | `prototypes/data-entry/project/scripts/docs/category_report.lua` + wiring in `data-entry.yaml` — server starts cleanly against the prototype |

Additional implementation notes:

- **`print()` routing**: redirected `print` from `os.Stdout` to `r.stdout` in `newRuntime`. Required for captured-stdout document rendering; CLI behavior unchanged (stdout is os.Stdout in that context anyway).
- **Print-redirect stayed minimal**: one helper `luaPrint` mirroring gopher-lua's base `print` semantics (tab-separated args, newline).
- **App refactor**: `newDocumentService` now takes `(scriptEngine, depsFunc)`. `rebindApp` in test helpers wires a default service so handler tests continue to work.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind

Observations:

- Architecture follows the existing action-mode pattern. `WithDocumentMode` mirrors `WithActionMode`; `Engine.ExecuteDocument` mirrors `ExecuteAction`. No new cross-subsystem abstractions.
- The `documentScriptEngine` interface is defined at the call site (in `internal/dataentry/document.go`) per CLAUDE.md's consumer-side interface rule.
- All errors surface to the HTTP response; the only intentional "silent-ish" path is the disk-cache write on command renders, which already logs a warning.
