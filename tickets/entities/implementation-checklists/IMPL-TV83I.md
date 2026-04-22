---
id: IMPL-TV83I
type: implementation-checklist
title: 'Implementation: Migrate fsstore write paths to RootedFS (closes CodeQL path-injection alerts)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code — `rooted_test.go` gains `TestRootedFS_OpenForWrite_*` (3 tests) and `TestRootedFS_AbsPath*` (2). `attachment_filename_test.go` audits sanitizer agreement.
- [x] Integration tests written (test full flow, not just units) — conformance suite + persistence + recovery + differential tests all exercise fsstore end-to-end through the new RootedFS-wired config. `watcher_internal_test.go` exercises self-echo LRU with the absolute-path contract preserved.
- [x] Happy path implemented — 5 write sinks (`writeDataFile`, `saveIndex`, `writeAttachment` buffered, `streamToFile`, data-file `Remove`) + all directory ops (`ReadDir`/`Stat`/`Walk`/`MkdirAll`/`Rename`) now flow through `*RootedFS`.
- [x] Edge cases from planning handled — `OpenForWrite` MemFS fallback via `RootedFS.SupportsStreaming`; `absPath` panics on programming error only; watcher absolute-path contract preserved.
- [x] Error handling in place — every resolve error from `RootedFS` propagates; buffered fallback happens only when streaming is unsupported (detected via `SupportsStreaming`, not via error sniffing).

## Test Quality

- [x] ~~Using fixture builders or factories for test data~~ (N/A: fsstore tests use Config literals; we consolidated the helper into `newConfig(fs)` and reuse it across 4 test files.)
- [x] No hardcoded values in assertions when object is in scope — filepath.Base tests compare derived outputs, not hardcoded strings.
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] Feature manually tested end-to-end — conformance suite is exactly "end-to-end for the store backend", includes attachment upload + delete + rename.
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified — `just lint`, `just test`, `just coverage-check` all pass; coverage 72.8% (up from 72.6%).

**Verification Evidence:**

All ACs verified:

- **AC1** (FSStore.rooted field + Config.Rooted required): `TestFSStore_New_RequiresRooted` covered by compile-time check + existing conformance. Config rewrite mechanically renames 4 fields to their *Key counterparts.
- **AC2** (5 write sinks routed through RootedFS): conformance suite passes; inspecting the diff shows `writeDataFile`, `saveIndex`, `writeAttachment`, `streamToFile`, and all data-file `Remove` sites now use `s.rooted`.
- **AC3** (data-file Removes migrated, attachment Removes on rooted too): by virtue of Option 2 (keys throughout), attachment removes also flow through rooted. `DeleteAttachment`, `removeAttachmentDir` all use keys. No raw `s.dirs.Remove` remaining.
- **AC4** (OpenForWrite exists and behaves): 3 new tests pass. MemFS test asserts rejection of bad key; OsFS test asserts write-read round-trip and nested parent creation.
- **AC5** (sanitizer agreement test): `TestAttachmentFilename_RootedFSAgreement` characterizes the gap. `filepath.Base` is the only pre-store sanitizer, and it strips path-traversal (`../../etc/passwd` → `passwd`). Filenames containing `:`, `\`, control chars, or Windows reserved stems (`CON`, `NUL`, etc.) are rejected by RootedFS at write time — a loud-failure behavior change documented in the test. No production regression observed; CLI/HTTP callers that pass such filenames today would have failed anyway (Windows would have opened a device handle; POSIX would have stored a weird filename).
- **AC6** (CodeQL alerts close): verify post-merge.
- **AC7** (CI green): local `just lint` → 0 issues, `just test` → all PASS, `just coverage-check` → PASS (72.8%).

## Quality

- [x] Code follows project patterns — matches TKT-0M8PM's RootedFS-pilot pattern; `absPath` helper mirrors similar "programmer error" panics in the codebase.
- [x] No security issues introduced — `resolve()` stays unexported; `AbsPath` documented as "don't pass back to raw FS"; `OpenForWrite` documented atomicity-is-caller contract.
- [x] No silent failures — `absPath` panics loud; `SupportsStreaming` branch is explicit rather than error-sniffed; attachment filename rejection surfaces as a write error.
- [x] No debug code left behind — reviewed diff; no print/log statements.

## Review findings (from `/design-review`)

12 findings raised pre-implementation. All addressed in the plan and code:

- **Critical (1)**: F1 streamToFile API — resolved via `RootedFS.OpenForWrite`. Keeps `resolve` unexported.
- **Significant (5)**: F2–F5, F7, F8 — all addressed. See PLAN-LEG21 "Design Review Findings" section.
- **Minor (3)**: F6 dropped the Dir→Key rename (then re-adopted for Option 2 where it's necessary). F9 effort held. F10 data-file Removes migrated fully.
- **Nits (3)**: documented and accepted.

Review-response entities created during code-review phase below.
