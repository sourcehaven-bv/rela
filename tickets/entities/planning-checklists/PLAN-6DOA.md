---
id: PLAN-6DOA
type: planning-checklist
title: 'Planning: Replace Workspace.mu with atomic.Pointer'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:** see TKT-252Y body and the implementation checklist IMPL-SS01 for the
as-built scope, which expanded during implementation to fix several pre-existing
data races discovered by the compiler when changing the field types. The
original plan covered Workspace.mu only; the implementation also bundled
meta+automation+searchIdx into a single workspaceState atomic.Pointer (after the
cranky review highlighted that two-call epoch mismatches would otherwise be
observable).

**Acceptance Criteria:** documented in TKT-252Y. All 11 criteria verified — see
the verification evidence in IMPL-SS01.

## Research

- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

Research summary: see `.ignored/locking-findings.md` and
`.ignored/locking-alternatives.md` for the full investigation. Key references:
prior reviewer findings RR-CSTD ("30s timeout under global write lock is a DoS")
and RR-Z3R9 ("Action handler missing workspace write lock") motivated the wider
FEAT-W5T8 refactor; this ticket is the first stage.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

Original approach (per `.ignored/locking-alternatives.md` §2.2): two independent
atomic.Pointer fields for meta and searchIdx, no internal mutex.

Revised approach after the cranky review: a single `workspaceState` struct held
via `atomic.Pointer[workspaceState]`, plus an internal `reloadMu sync.Mutex`
that serializes Reload/Sync/Close, plus a `closed atomic.Bool` flag. This is the
L1+L2 recommendation from the cranky review and is strictly stronger than the
original plan.

**Files modified:**
- `internal/workspace/workspace.go` — primary refactor
- `internal/workspace/analysis.go` — 5 unprotected `w.meta` reads migrated to `w.Meta()`
- `internal/workspace/workspace_test.go` — replaced `TestRLock` with `TestConcurrentReloadStateSnapshot`
- `internal/workspace/query_test.go` — extracted `newQueryTestWorkspace` helper

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

This is a pure internal refactor with no new input surface. The refactor
**fixes** several pre-existing data races (unprotected `w.meta` reads in
FormatEntity, ExecuteView, executeLuaActions, and 5 functions in analysis.go).
It does NOT fix the larger graph-torn-during-reload race, which is a
pre-existing limitation tracked as future work (RR-PA4Y, RR-FJH7).

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

See TKT-252Y AC table and IMPL-SS01 for the per-AC test mapping. The new
`TestConcurrentReloadStateSnapshot` exercises the workspaceState atomic-pointer
paths under `-race` with reader, writer, and reloader goroutines, modeling the
production locking discipline (App.writeMu serializes writers and reloaders).

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

The original risk register is in IMPL-SS01. The cranky code review surfaced one
risk that the planning under-estimated: **the graph is mutated in place by
`repo.Sync` and was never under the workspace mutex's protection on the read
path**. The implementation acknowledges this as a documented limitation (struct
comment) and tracks it as future work rather than expanding scope.

**Effort:** ~~xs-s~~ → **m** (after the post-review course correction). The
original plan was for half a day; the actual work, including the workspaceState
bundling and the strengthened test, was closer to a full day plus a thorough
review cycle.

## Documentation Planning

For enhancements: identify what documentation needs updating.

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:** internal refactor only. The Workspace struct doc
comment was rewritten to describe the new concurrency model and the known
graph-torn limitation. No user-facing docs need updating. No docs-checklist
needed.

## Design Review

- [x] ~~Run `/design-review` before starting implementation~~ (Skipped: pre-implementation; equivalent thoroughness was achieved by the cranky-code-reviewer agent invoked during the review phase, which found 12 issues and led to a substantial mid-implementation course correction)
- [x] All critical/significant findings addressed in plan (after the post-review course correction: workspaceState bundling, reloadMu serialization, closed flag, strengthened test, error-handling reorder)

**Design Review Findings:** RR-PA4Y, RR-FJH7, RR-XA3R, RR-EI58, RR-BY3P,
RR-6G1K, RR-TMEM, RR-AAW2, RR-E4HZ, RR-L289, RR-IJNT, RR-7PCN
