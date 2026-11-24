---
id: REV-BLTPTDH
type: review-checklist
title: 'Review: Give the whole-set scopedHeaders callers their own read function'
started: "2026-11-24"
completed: "2026-11-24"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`) (changed packages dataentry and archguard on default, memorybackend and sqlite builds; full suite in CI)
- [x] Lint clean (`just lint`) (golangci-lint on changed packages, arch-lint, plimsoll)
- [x] Comment lint gate clean (`just comment-lint`)
- [x] ~~Coverage maintained (`just coverage-check`)~~ (N/A locally: CI enforces package floors; this change only adds tests)

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [x] All critical review-responses addressed
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** RR-75K7M04, RR-Q1N5EI6, RR-YC298WN, RR-H3MM3O3, RR-DVXPRLC
(design); RR-QFA08UH, RR-M5LOLE0, RR-XGDNK9C, RR-O84GK5B, RR-RMIYWH3, RR-W3AHRHN
(code). Security review: no findings.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**
- scopedHeaders serves bounded reads only: PASS (TestScopedHeaders_UnboundedReadIsRefused).
- Each whole-set caller has its own allowlist entry naming a ticket: PASS (TestUnpagedReadAllowlist_RequestPathNamesItsTicket).
- Scans take their query from scopedQuery: PASS (TestScanHeaders_TakesItsQueryFromScopedQuery, TestScanCallProblem).
- Window cut after the row gate: PASS (TestScopedHeaders_WindowReadsOneWindow).
- Bounded paths are scale-free: PASS (TestLLMAction_UnconditionedCandidatesScaleFree, TestDynamicCollections_ReadIsScaleFree).

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] ~~Docs-checklist created and linked via `has-docs`~~ (N/A: internal refactor)
- [x] ~~User-facing documentation updated~~ (N/A: internal refactor)
- [x] ~~Docs-checklist marked as done~~ (N/A: internal refactor)

**Docs Checklist:** N/A

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI
