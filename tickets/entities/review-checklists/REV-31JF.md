---
id: REV-31JF
type: review-checklist
title: 'Review: Extract automation.Runner with consumer-side Host interface'
status: in-progress
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [ ] All tests pass (`just test`)
- [ ] Lint clean (`just lint`)
- [ ] Coverage maintained (`just coverage-check`)

## Code Review

- [ ] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [ ] All critical review-responses addressed
- [ ] All significant review-responses addressed
- [ ] Self-reviewed the diff for unrelated changes

**Review Responses:** <!-- List IDs of review-response entities created, e.g.,
RR-xxxx -->

## Acceptance Verification

- [ ] Each acceptance criterion tested (reference planning checklist)
- [ ] Test evidence documented in implementation checklist

**Acceptance Status:**
<!-- For each acceptance criterion, state PASS/FAIL with evidence -->

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [ ] Docs-checklist created and linked via `has-docs`
- [ ] User-facing documentation updated
- [ ] Docs-checklist marked as done

**Docs Checklist:** <!-- e.g., DOCS-xxxx -->

## Final Checks

- [ ] Commit message explains the why, not just what
- [ ] No TODOs or FIXMEs left unaddressed
- [ ] Ready for another developer to use

## Pull Request

- [ ] Run `/pr` command to create PR and monitor CI
- [ ] All CI checks pass
- [ ] PR URL documented below

**PR:** <!-- e.g., https://github.com/org/repo/pull/123 -->
||||||| b159dac
=======
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`)
- [x] Lint clean (`just lint`)
- [x] Coverage maintained (`just coverage-check`)

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [x] All critical review-responses addressed
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** Two parallel review rounds were run (cranky-code-reviewer +
go-architect) against both the plan (round 1) and the shipped implementation
(round 2). Findings were addressed inline via commits `4bc17e4`, `e3d6f7d`,
`e017972`, `4315df6`, `12747ad` rather than tracked as `review-response`
entities. Full breakdown — 1 critical, 5 significant, 6 minor — is in
PLAN-V6UR "Design Review Findings (round 2)".

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

All 7 acceptance criteria from PLAN-V6UR pass:

- AC1 — `autocascade.Runner` exists with 7-method `Host`: PASS (see `internal/autocascade/host.go`, `runner.go`).
- AC2 — Workspace dispatch goes through Runner: PASS (`grep -r applyAutomationSideEffects internal/workspace` returns nothing).
- AC3 — 9 non-Lua cascade tests pass unchanged: PASS.
- AC4 — 5 `TestLuaAutomation_*` pass against real engine: PASS.
- AC5 — 10 new Runner unit tests: PASS.
- AC6 — `MaxDepth = 50` lives in `autocascade`, not `workspace`: PASS.
- AC7 — `just ci` green: PASS (except the `Rela Tickets` gate this checklist itself unblocks).

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] ~~Docs-checklist created and linked via `has-docs`~~ (N/A: pure internal refactor, no user-facing surface)
- [x] ~~User-facing documentation updated~~ (N/A: pure internal refactor; CLAUDE.md updated in-tree with the new "Consumer-side interfaces" pattern section using `autocascade.Host` as the worked example)
- [x] ~~Docs-checklist marked as done~~ (N/A: no docs-checklist needed)

**Docs Checklist:** N/A (internal refactor)

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI
- [x] All CI checks pass
- [x] PR URL documented below

**PR:** https://github.com/sourcehaven-bv/rela/pull/694
>>>>>>> origin/develop
