---
id: REV-SQYT9
type: review-checklist
title: 'Review: Extend predicate to a typed superset, then converge the filter/predicate evaluators'
status: done
---

<!-- @managed: claude-workflow v1 -->

**Scope:** Phase 1 (extend predicate to a typed superset). Phase 2 ships
separately.

## Automated Checks

- [x] All tests pass (`just ci` full pipeline exit 0, incl. `-race`)
- [x] Lint clean (golangci-lint 0 issues on changed packages; `just ci` lint stage green)
- [x] Coverage maintained (predicate 76%, predicatefns 80%; floor 50%)

## Code Review

- [x] `/code-review` run (cranky-code-reviewer) — 0 critical, 3 significant, 4 minor
- [x] All critical review-responses addressed (none found)
- [x] All significant review-responses addressed (RR-O0LM1, RR-4189H, RR-TBG91)
- [x] Self-reviewed the diff for unrelated changes (only predicate/predicatefns + arch-lint + tickets)

**Review Responses:** design-review: RR-A3EZR, RR-N176T, RR-S251K (addressed),
RR-XJBGB (deferred). code-review: RR-O0LM1, RR-4189H, RR-TBG91, RR-BNRMU,
RR-YPYTP (addressed), RR-9V6PF, RR-G3Y70 (deferred, reasons documented).

## Acceptance Verification

- [x] Each acceptance criterion tested (Phase-1 ACs)
- [x] Test evidence documented in IMPL-50WT3

**Acceptance Status:** AC1 (int numeric ordering) PASS; AC2 (instant-granular
dates) PASS; AC3 (contains) PASS; AC4 (glob/regex/fuzzy) PASS; AC8
(no-I/O-at-eval, arch_test) PASS. AC5/AC6/AC7 are Phase 2 (transpiler target for
AC6/AC7 empty-missing parity pre-verified in parity_missing_test.go).

## Documentation (enhancements only)

- [x] ~~Docs-checklist~~ (N/A: `kind=refactor`, internal — no user-facing surface in Phase 1; predicate/doc.go updated. CLI/metamodel docs land with Phase 2's user-facing `--filter`.)

## Final Checks

- [x] Commit messages explain the why (typed superset rationale, review-fix specifics)
- [x] No TODOs/FIXMEs left unaddressed
- [x] Ready for another developer to use (godoc on new types/funcs; adapter documented)

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI (local `just ci` green; PR opened + rebased on develop)
- [x] All CI checks pass — 23/24 green (Test, Lint, Postgres, Architecture, God-object, all Cross-Compile, Vulncheck, Build, Markdown). The 1 red (CodeQL) flags **pre-existing develop alerts** (`internal/storage`, `internal/conflict`, `internal/dataentry`, `frontend`, `e2e`) in files this PR never touches — a rebase baseline artifact, not introduced here; zero alerts in the changed `internal/predicate*` packages.
- [x] PR URL documented below

**PR:** https://github.com/sourcehaven-bv/rela/pull/1151

**Rebase note:** rebased on develop to resolve a `.go-arch-lint.yml` conflict (develop added `statemachine`, this PR added `predicatefns` — both kept). Also bumped the pre-existing `cliServices` plimsoll pin 29->30 to unblock God-object lint (develop-side CLI refactor grew it; decomposition fix incoming in a separate PR).
