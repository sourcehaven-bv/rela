---
id: REV-L9MJWT
type: review-checklist
title: 'Review: Gantt perf: header projection + SQL-scoped subtree drill'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`)
- [x] Lint clean (`just lint`)
- [x] Comment lint gate clean (`just comment-lint`)
- [x] ~~Coverage maintained (`just coverage-check`)~~ (N/A: package floors
      unchanged by this diff; the full dataentry suite runs green and the new
      code carries dedicated tests, including a property test)

golangci-lint 0 issues (after gocognit decomposition + shadow/intrange
fixes); arch-lint clean (documented `.ignored/` exclude); comment-lint gate
clean; full dataentry suite green.

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [x] All critical review-responses addressed
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** RR-YRD34H (critical — silent Depth clamp truncated the
drilled fold; fixed with the iterative closure + deep-chain test), RR-G9IETK
(critical — multi-parent/policy equivalence breaks; fixed with
decline-on-divergence + property test), RR-SULIE9 (significants bundle — all
fixed). Reviewer finding #12 (the store API's silent clamp footgun) spun off
as TKT-VAD46Q rather than widened into this ticket.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

AC1 equivalence: `TestGantt_SubtreeDrillMatchesFullBuild`,
`TestGantt_SubtreeDrillExternalParentMatchesFull`, and
`TestGantt_SubtreeDrillPropertyEquivalence` (8 seeded random graphs, every
node drilled, byte-equality). AC2: full prior suite green. AC3: re-measured
post-fix — full 387→225 ms (1.7×), drill 327→70 ms (4.7×) at 50k. AC4:
uniform-404 tests green on the fast path.

## Documentation (enhancements only)

- [x] ~~Docs-checklist created and linked via `has-docs`~~ (N/A: no
      user-facing behaviour change — same wire contract, faster; the one
      documented semantic nuance, subtree-scoped cycle diagnostics, lives in
      the endpoint godoc where its audience is)
- [x] ~~User-facing documentation updated~~ (N/A: as above)
- [x] ~~Docs-checklist marked as done~~ (N/A: as above)

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI (lands on the open
      PR #1472 for TKT-MW28U5 — same feature arc, same branch)
