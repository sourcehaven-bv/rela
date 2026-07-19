---
id: REV-UJBEGC
type: review-checklist
title: 'Review: Badge colors never resolve when a property''s name differs from its custom-type name'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`)
- [x] Lint clean (`just lint`)
- [x] Coverage maintained (`just coverage-check`)

`just test` + `just lint` (0 issues) + `just coverage-check` (package 50% /
total 65% thresholds satisfied, total 76.1%) all passed. After the review fixes,
the frontend suite re-ran green (1340 tests), typecheck clean, ESLint 0 errors,
jscpd no new duplication; the review fixes touched no Go code.

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [x] All critical review-responses addressed
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** RR-BTUN4L (significant, addressed), RR-OCXFPC
(significant, addressed), RR-9J59FE (minor, addressed), RR-0VUEF6 (minor,
addressed), RR-HOMQAU (nit, addressed), RR-7C1E3G (nit, wont-fix with reason).
No critical findings.

Notable: the reviewer's EntityDetail wiring suggestion (part of RR-BTUN4L) was
investigated and rejected with evidence — those cells pass the property's TYPE
name (`cell.propType`) as `:property`, which is already the styles key; the
direct-key fallback resolves it and is now documented + test-pinned.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

- Property ≠ type resolves configured color: PASS (Badge.test.ts + schema.test.ts cases; live browser check on tickets/ project — 94 badges colored per data-entry.yaml styles).
- Property-name/direct-key fallback preserved: PASS (existing tests unchanged and green; new EntityDetail PropType pin test).
- entityType disambiguation: PASS (store + component tests; widgets now forward entity-type).
- No regression in labels/normalization/gray default: PASS (full suite 1340 green; RR-UD2D non-determinism guard untouched).

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] ~~Docs-checklist created and linked via `has-docs`~~ (N/A: bug fix)
- [x] ~~User-facing documentation updated~~ (N/A: behavior now matches the already-documented `styles:` config)
- [x] ~~Docs-checklist marked as done~~ (N/A: bug fix)

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI
- [x] All CI checks pass
- [x] PR URL documented below

**PR:** in flight — /pr running now (done-gate requires the bug done +
validation clean before the PR exists); URL recorded here as soon as it is open
and CI is green.
