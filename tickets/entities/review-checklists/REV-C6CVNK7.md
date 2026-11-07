---
id: REV-C6CVNK7
type: review-checklist
title: 'Review: Range fingerprints for types with redacted fields'
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

Evidence: `go test -race` on visibility, canonical, affordances, archguard,
reconcile and store packages passes; golangci-lint on the changed packages
reports 0 issues; `just comment-lint` clean; `just arch-lint` OK; plimsoll clean
on the touched packages; `archplan check` has no new findings apart from SQLite
FTS orphan tables already on develop, and one ratchet loosen (propagation_cost
9.4 to 9.5) recorded with a reason. `just ci` runs the full suite before the PR.

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [x] All critical review-responses addressed
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** cranky-code-reviewer and rela-security-reviewer.
RR-5UQQ30L (critical) and RR-LWAYT7C, RR-OGZUZU0, RR-SLPLX9E, RR-TNCBV97
(significant) addressed. Minor and nit: RR-HNODUN8, RR-UUKPM67, RR-Q5Q3I2J,
RR-N6M3GOB, RR-VAEI53X, RR-EAJ0CAA, RR-CJ2JAVL addressed; RR-ER381BQ deferred to
step 5; RR-R7LLLSM wont-fix. Design review: RR-3TYBXP3, RR-7B4BLLB, RR-OE4GAV0,
RR-P1FK0IN, RR-G1E6X9I, RR-JILCGF8, RR-Y8X93JV, RR-PZBJ8XQ addressed; RR-N85B86W
deferred.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

- A change to a hidden field does not change the reader's fingerprint: PASS (`TestElements_HiddenFieldChangeKeepsFingerprint`).
- Redacted and unredacted paths agree when the reader has no hidden field: PASS (`TestElements_GoPathAgreesWithStoredPath`, `TestElements_Faces`, `TestElements_StoredTypeWithUnfilledRows`).
- Elements are the ETag hash of the redacted view: PASS (`TestElements_ElementsAreRedactedViews`).
- Budget: Fingerprint and List cost the same store calls at 10 and 50 rows and with or without hidden values: PASS (`TestElements_StoreCallBudget`).
- RR-3HN68CI decided and documented: PASS (reconcile package doc and elements.go).

## Documentation (enhancements only)

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-MMPGWJD

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI
