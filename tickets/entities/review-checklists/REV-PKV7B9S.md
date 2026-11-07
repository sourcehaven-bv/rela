---
id: REV-PKV7B9S
type: review-checklist
title: 'Review: Reconciliation element and reader view hash'
started: "2026-11-07"
completed: "2026-11-07"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`)
- [x] Lint clean (`just lint`)
- [x] Comment lint gate clean (`just comment-lint`)
- [x] Coverage maintained (`just coverage-check`)

`just ci` passed locally (exit 0): tests, lint, arch-lint, comment-lint and
coverage. `reconcile` is at 100% coverage. `just comment-report` showed one
restatement finding in the new code (Range.Add); fixed by documenting the edge
behaviour of Add and Remove.

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [x] All critical review-responses addressed
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** Design review: RR-R6ZK1SK, RR-I6TTK56, RR-IIGONJR,
RR-3HN68CI (deferred to step 4 with reason), RR-U081VJ9, RR-2BV7V1O, RR-QYKY2XH,
RR-XDCLMHW, RR-IX7IWMK, RR-8CQMKV1, RR-MY0HTGC, RR-LE3SX9E. Code review (cranky
+ rela-security-reviewer): RR-4YNQ1PZ, RR-RC82FNT, RR-PAR9PDO, RR-MDMA4S5
(deferred: wire codecs belong to step 5), RR-KIPJ213, RR-5GKT001, RR-9L6HKML,
RR-O4DBSCO, RR-GG024VM, RR-HO8TD8W. No critical findings; every significant
finding is addressed except RR-3HN68CI, deferred with a recorded reason.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**
1. One view-hash function used by the ETags: PASS (`TestRelationETag_IsViewHash`,
`viewETag` used by both ETags, manual check on a running server).
2. Redacted field does not change the view hash: PASS
(`TestElement_RedactedFieldDoesNotChangeViewHash`,
`TestEntityETag_HiddenEqualsAbsent`, `TestEntityETag_ExcludesHiddenFields`).
3. Fingerprint pinned: PASS (`TestFingerprint_Golden`, recomputed in Python).
4. Additive ranges: PASS (`TestRange`, `TestFingerprint_Arithmetic`,
`TestFingerprintFromLimbSums_MatchesAdd`).

## Documentation (enhancements only)

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-FYXOOYB

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI
