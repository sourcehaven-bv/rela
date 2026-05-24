---
id: REV-R60I
type: review-checklist
title: 'Review: Lua read bindings still use context.Background() — partial cancellation'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`) — `ok github.com/Sourcehaven-BV/rela/internal/lua` plus full suite green.
- [x] Lint clean (`just lint`) — `0 issues.` after fixing gofmt + revive context-as-argument finding.
- [x] Coverage maintained (`just coverage-check`) — `Total test coverage: 76.9% (16346/21250)`, all floors PASS.
- [x] Arch lint clean (`just arch-lint`) — `OK - No warnings found`.

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent) — invoked via Agent tool. 11 findings (1 critical, 2 significant, 4 minor, 4 nit).
- [x] All critical review-responses addressed (RR-E78K).
- [x] All significant review-responses addressed (RR-LCM5 addressed; RR-4XS8 deferred with justification + follow-up TKT-FVQ4 filed).
- [x] Self-reviewed the diff for unrelated changes.

**Review Responses:** RR-E78K (critical, addressed), RR-4XS8 (significant,
deferred → TKT-FVQ4), RR-LCM5 (significant, addressed), RR-7U05 (minor,
addressed), RR-EU51 (minor, addressed), RR-4PX1 (minor, addressed), RR-QDV6
(minor, addressed), RR-2RK4 (minor, addressed), RR-YI8W (nit, addressed),
RR-R3VU (nit, addressed), RR-UCJL (nit, addressed).

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

- **AC1 — No `context.Background()` in Lua bindings:** PASS.
`grep "context.Background" internal/lua/runtime.go` returns three hits: L134
(callerCtx doc), L142 (callerCtx fallback), L552 (applyTimeout fallback). No
binding hits.

- **AC2 — Parent ctx flows through read bindings:** PASS.
`TestReadBindings_UseCallerContext` (7 subtests) verifies each binding's
collaborator call carries the parent marker. Empirical regression check:
reverting either L1264 or L1270 individually now produces named, targeted
failures (e.g. `call Searcher.Search used a ctx without the parent marker`).

- **AC3 — No regression:** PASS.
`just test` → all packages green; `just lint` → 0 issues; `just coverage-check`
→ all floors PASS at 76.9% overall.

## Documentation (enhancements only)

Skipped per planning. No user-facing API change; Lua scripts don't observe ctx
behavior.

- [x] ~~Docs-checklist created and linked via `has-docs`~~ (N/A: planning explicitly marked docs out of scope)
- [x] ~~User-facing documentation updated~~ (N/A)
- [x] ~~Docs-checklist marked as done~~ (N/A)

**Docs Checklist:** N/A

## Final Checks

- [x] Commit message explains the why, not just what (to be authored when committing)
- [x] No TODOs or FIXMEs left unaddressed (the cancellation-swallow defect is a separate ticket TKT-FVQ4, not a TODO)
- [x] Ready for another developer to use — binding API is unchanged, only internal ctx threading.

## Pull Request

- [x] ~~Run `/pr` command to create PR and monitor CI~~ (Deferred: this `/ticket` workflow ends at the done transition; the user runs `/pr` separately when ready to ship.)
- [x] ~~All CI checks pass~~ (Deferred: belongs with the PR step.)
- [x] ~~PR URL documented below~~ (Deferred: belongs with the PR step.)

**PR:** TBD (to be created via `/pr` in a separate workflow step)
