---
id: REV-A0A311
type: review-checklist
title: 'Review: Relation-picker autosave aborts with "unknown types" when the linked entity is outside the 100-candidate window'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`)
- [x] Lint clean (`just lint`)
- [x] Comment lint gate clean (`just comment-lint`)
- [x] Coverage maintained (`just coverage-check`)

`just ci` exit 0. Re-run after the review fixes: frontend 2481/2481 unit tests
pass, `npm run typecheck` clean, `npm run lint` 0 errors (124 pre-existing
warnings), full e2e suite 292 passed / 8 skipped / 0 failed. `just comment-lint`
— no unresolvable doc links across 14166 comments. `just coverage-check` —
package and total thresholds PASS, total 79.7%. This change is frontend-only, so
Go coverage is unmoved.

**Comment findings.** None introduced. One comment was *removed* as a finding of
this review (RR-L7RANS): a stale claim in `handleEntityCreated` that the fix had
falsified.

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [x] All critical review-responses addressed
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** RR-L7RANS, RR-0RAD74 (critical, both addressed);
RR-FBI16Z, RR-S5J12Y (significant, addressed); RR-IT4HSP, RR-A049JA
(significant, deferred to TKT-MG2FXQ / TKT-QF41FL); RR-6RI00V (minor,
addressed); RR-IFEWIB (nit, wont-fix with measurement).

Both critical findings were verified in the source before acting, not taken on
trust. The two deferred significant findings are behaviour changes to shared
paths that the reviewer agreed should not ride along with a bug fix; each has a
filed ticket rather than a note.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

- *Adding a second value to a relation whose existing link is beyond the first
candidate page persists both edges* — **PASS**.
`e2e/tests/relation-picker-large-candidate-set.spec.ts`, verified red before the
fix (`["FEAT-001","FEAT-104"]`, no PATCH, exact reported toast) and green after.
Confirmed independently in a real browser: both chips render, a PATCH is sent,
no toast, server holds both edges.
- *The pre-existing off-page link is not dropped by the save* — **PASS**, second
assertion in the same spec.
- *Paged and single-page cache entries never satisfy each other* — **PASS**,
`entities.test.ts`, mutation-verified (removing the mode from the key fails it).
- *Paged entries are still reached by write invalidation* — **PASS**,
mutation-verified (reordering the key to `<mode>:<type>:` fails it, and also
fails a pre-existing invalidation test).

## Documentation (enhancements only)

Skipped — this is a bug fix with no user-facing surface change. The one
behavioural addition is a `console.warn` on candidate truncation, which is
operator diagnostics, not documented UI.

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

Three forms.spec.ts failures observed during verification were investigated and
are **pre-existing flakes, not a regression**: with this change stashed, a
different test in the same file failed, and `forms.spec.ts --repeat-each=2`
passes 32/32. Worth noting rather than burying, but out of scope here.

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI
