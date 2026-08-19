---
id: REV-1HTU0H
type: review-checklist
title: 'Review: Consolidate cardinality analyzers; stop swallowing CountRelations errors'
status: in-progress
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`go test ./...` clean, 2026-08-19, re-run after review fixes)
- [x] Lint clean (`just lint` 0 issues; `just arch-lint` OK; `just plimsoll` clean)
- [x] Coverage maintained (`just coverage-check`: floors + total 77.1% >= 65% PASS; internal/analysis 76.9%)

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent; reviewer also ran a 3000-case randomized differential test against the reconstructed pre-refactor functions — zero mismatches)
- [x] All critical review-responses addressed (none found)
- [x] All significant review-responses addressed (RR-4JWU2I, RR-STCCY8, RR-5FKMKX — all fixed)
- [x] Self-reviewed the diff for unrelated changes (touches only internal/analysis + the three CLI call-site files)

**Review Responses:**

RR-4JWU2I (significant, addressed: honest godoc on the validate banner),
RR-STCCY8 (significant, addressed: CLI-boundary error tests; double-scan
hazard documented in-code, compute-once is the follow-up), RR-5FKMKX
(significant, addressed: relName folded into cardinalitySpec), RR-1HB83A
(minor, addressed: direction in the count error), RR-3ZJM3P (minor,
addressed: load-bearing comments), RR-TFUP2X (minor, addressed:
interleaving-regression pin + import nit). Reviewer leverage notes not
turned into findings: promote the fault-injection wrapper to storetest +
an Nth-call failure variant (deferred — cross-package test infra, own
change); the MCP fifth cardinality copy (already flagged to the
architect, out of scope by recorded decision).

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

- AC1 (identical violations/ordering/labels) — PASS: pinning tests written first against the old code, unchanged after; plus the reviewer's 3000-case differential run (zero mismatches, incl. []-not-null JSON shape).
- AC2 (max=0 meaningful, min=0 skip) — PASS: TestCheckCardinality_BoundEdgeCases.
- AC3 (count error fails loudly, no fabricated violations) — PASS: TestCheckCardinality_CountErrorFailsLoudly (+ AnalyzeAll propagation subtest).
- AC4 (CLI surfaces error as command failure, no partial output) — PASS: TestAnalyzeCmds_CountErrorAborts (empty output buffer asserted for analyze cardinality + analyze all); validate path compiler-enforced + green cli tests.

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] ~~Docs-checklist created and linked via `has-docs`~~ (N/A: refactor kind — section skipped per template; behaviour contract unchanged for users)
- [x] ~~User-facing documentation updated~~ (N/A: no user-facing docs affected; error policy documented in godoc + ticket)
- [x] ~~Docs-checklist marked as done~~ (N/A)

**Docs Checklist:** N/A (refactor)

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [ ] Run `/pr` command to create PR and monitor CI
- [ ] All CI checks pass
- [ ] PR URL documented below

**PR:** <!-- e.g., https://github.com/org/repo/pull/123 -->
