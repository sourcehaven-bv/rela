---
id: REV-G1TJ3C
type: review-checklist
title: 'Review: Guard test forbids zero-face reads outside a shrinking allowlist; faced fixtures by default'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`)
- [x] Lint clean (`just lint`)
- [x] Comment lint gate clean (`just comment-lint`)
- [x] Coverage maintained (`just coverage-check`)

Notes: `just test` passed for every package except one run where
`internal/dataentry` hit the 10-minute race timeout in the untouched
`TestAnalyzeProperties_StopsScanningAtCap` on a machine loaded by parallel
agents; the package passes under `-race` with a longer timeout, and the test
passes alone. `just lint` was run as `golangci-lint run
--allow-parallel-runners` because other worktrees held the lint lock: 0 issues.
Coverage: the change adds only test files, and the gate-pin subtests keep the
previously covered handler branches exercised; CI's coverage job confirms.

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [x] All critical review-responses addressed
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** Design: RR-8TG4HL, RR-HSD3CR, RR-BGQYUA (addressed). Code:
RR-Y55W04, RR-FD39ON, RR-0LDG0R (significant, addressed); RR-TGZZ70, RR-L21II4,
RR-UC7OF1, RR-Q02K2Q (minor, addressed); RR-OV96OW, RR-RPH6CC (minor, wont-fix
with reason); RR-TIKY22 (nit, addressed); RR-ZXOFM0 (nit, wont-fix with reason).

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**
1. New `GetEntity(ctx, id)` in a non-allowlisted file fails: PASS (`TestScanTree_FindsSyntheticViolation`, `TestDiffAllowlist/new_file_fails`).
2. Over and under counts fail, so the list stays exact: PASS (`TestDiffAllowlist` over/under/gone cases).
3. The real tree matches the allowlist: PASS (`TestNoNewZeroFaceReads`; 73 files, 120 reads).
4. No test seeds a zero-face row for a type that declares faces; `seedDraftAndPublishedTicket` deleted: PASS. The gate-pin fixture seeds a zero-face row only on the faceless `ticket` type of `newTestAppV1`.

## Documentation (enhancements only)

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-S4V499 (CLAUDE.md "Don't add a zero-face read").

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] ~~Run `/pr` command to create PR and monitor CI~~ (N/A: this staged programme opens the PR with `gh pr create --base faces-intrinsic` after the ticket is done, and CI is monitored there)
