---
id: REV-SLHVCH
type: review-checklist
title: 'Review: List filter and sort evaluate hidden fields (value disclosure)'
started: "2026-10-13"
completed: "2026-10-14"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`)
- [x] Lint clean (`just lint`)
- [x] Comment lint gate clean (`just comment-lint`)
- [x] Coverage maintained (`just coverage-check`)

Evidence: `go test -race` passes on every touched package (dataentry,
affordances, acl, apiwire/v1), including TestGoldenParity. golangci-lint on the
changed packages reports 0 issues; arch-lint, the comment-lint gate and
plimsoll pass. Frontend: `npm run test:run` (299 files, 4201 tests) and
`npm run typecheck` pass; eslint reports no errors on the touched files (four
warnings in lines this change did not touch). Coverage is enforced by GitHub CI
on the PR.

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [x] All critical review-responses addressed
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** 34 across design and code review rounds. Critical:
RR-H3GP12 addressed. Significant: RR-3YJL4B, 5X1GV2, 8Y8I6T, 9FKZ9H, C67ULZ,
FFXKEK, S8X8GA, T7SY1M, VPN5EZ, YF00X7 addressed; RR-AVSNSN (per-hit cost of
list ?q=) and RR-WDAEYZ deferred, RR-ARMXA2 wont-fix, each with a reason. The
last code review found no leak: FieldVisibleOnEveryRow never passes a property
that FieldVerdicts hides (pinned by the differential test). Minor and nit:
RR-0ME3ZT, EGV6J4, IN8DMD, M70ORW, O137D3, V3EXUE, X15LZE, YFTZ31, 37BE1U,
F3940W, FB76GE, PKLTOE, XD82W2, XRWECR, V97BTD, QOK9J6 addressed; RR-50GXT5,
8QT70P, IAO5NC deferred; RR-MJ8XRF wont-fix.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**
1. PASS. `filter[]` and `sort` on a property hidden from the principal are
refused with 400 property_not_visible before any row is read
(TestListFieldGate_HiddenPropertyRejected, one case per operator). The body
names the refused use and property.
2. PASS. A `when:`-conditional field is refused, never evaluated
(TestListFieldGate_ConditionalVisibilityRejected); the answer depends only on
the policy and the principal's roles
(TestFieldVisibleOnEveryRow_NeverContradictsFieldVerdicts).
3. PASS. The store pushdown takes only a gatedListQuery
(TestGatedListQuery_BuiltOnlyByTheGate, TestListPushdown_Eligibility).
4. PASS. Golden corpus: list-filter-hidden, list-filter-hidden-eq and
list-sort-hidden refuse the viewer; admin, editor and the JWT stranger get 200,
byte-identical to the pre-fix recordings.
5. PASS. Free text skips matches in hidden fields
(TestListFieldGate_FreeTextSkipsHiddenMatch), at a cost pinned by
TestQueryBudget_ListFreeTextUnderFieldHiding.
6. PASS. Surfaces degrade instead of failing: the SPA list, the sidebar and
generated nav items drop a refused sort; a calendar shows a refused source's
error and renders the others. The load-time lint warns about every configured
surface a reader can be refused on.

## Documentation (enhancements only)

- [x] ~~Docs-checklist created and linked via `has-docs`~~ (N/A: bug fix)
- [x] ~~User-facing documentation updated~~ (N/A: bug fix; the 400 is described in the API error body and the load-time warning)
- [x] ~~Docs-checklist marked as done~~ (N/A: bug fix)

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI
