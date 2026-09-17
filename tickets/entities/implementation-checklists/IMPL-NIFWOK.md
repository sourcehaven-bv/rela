---
id: IMPL-NIFWOK
type: implementation-checklist
title: 'Implementation: Denied document stays on screen after a failed SSE re-render'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code
- [x] ~~Integration tests written (test full flow, not just units)~~ (N/A: the
server side of the flow is already covered by
`TestACLDocuments_GatesHiddenEntity`, which pins the 404 this fix reacts to; the
change itself is entirely client-side render behaviour)
- [x] Happy path implemented
- [x] Edge cases from planning handled
- [x] Error handling in place (errors surfaced, not swallowed)

`shouldDropHeldContent` in `frontend/src/api/errors.ts` classifies a rejection
as "the content a view is holding is no longer the principal's to see"
(401/403/404) rather than "the request failed to complete". Both document
loaders clear `docContent` in that case and are otherwise unchanged. The toast
and script-error panel still fire exactly as before — the change is that a
denial additionally removes the content.

Edge cases handled: a denial arriving from a *superseded* render must not blank,
since it concerns a document no longer displayed; the existing generation fence
already returns before the new branch, and a test pins it.

`isCached` is deliberately not cleared. The badge renders inside the
`v-else-if="docContent"` branch that blanking already unmounts, and every
successful render reassigns the flag, so a stale `true` has no path to the
screen — see the review-checklist for how the first draft got this wrong.

## Test Quality

- [x] Using fixture builders or factories for test data
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

A `denial(status)` helper builds the `ApiError` shape the axios interceptor
delivers, with its message derived from the status rather than a fixed string.
`problemAt(status)` builds the classifier fixture; it exists because
`normalizeApiError` prefers the `ProblemDetail` body's `status` over the HTTP
one, and the first draft reused a fixture pinned at 422, so three tests failed
for the fixture's reasons rather than the code's. Status cases are `it.each`
rows in all three files.

`DocumentsPanel.denial.test.ts` opens with a test that the panel renders at all,
because every other assertion in it is about content *disappearing* — a harness
that silently mounted nothing would pass them all.

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

Mutation testing is the load-bearing evidence, measured against the 52 tests in
the three touched files:

| Mutation | Failures |
|---|---|
| Remove the branch from `DocumentView.loadDocument` | 3 |
| Remove the branch from `DocumentsPanel.loadDocument` | 3 |
| Narrow the classifier to 403 only | 6 |
| Widen the classifier to include 500 | 1 |
| Drop the classifier's `instanceof` guard | 3 |
| Hoist the blanking above the generation fence | 1 |

Each mutant was applied and reverted individually with the suite re-run; the
baseline is green before and after. The transient-failure tests pass under every
mutation, confirming they do not mask the denial assertions.

Full frontend suite: 2747 tests / 169 files pass.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind

Follows `api/errors.ts`'s own stated convention — that file documents the
catch-site helpers (`getErrorMessage`, `getScriptError`) as the way a catch site
asks a question about a rejection instead of branching on shape.
`shouldDropHeldContent` is the third such helper and carries a `Nil:` contract
tag per the commentlint convention.

DRY: the classifier is shared rather than duplicated, and its rationale lives in
one godoc with the call sites pointing at it — the first draft duplicated ten
lines of prose across both components, which is the `duplication` pattern
commentlint tracks. The loaders themselves remain deliberately un-extracted, per
BUG-DJZTRF's implementation checklist; that judgement is unchanged and this fix
is 3 lines in each. A `useDocumentRender` composable would make the third copy
impossible and is worth doing, but it is a refactor of working code and does not
belong in a security fix.

`npm run typecheck` clean, `npm run lint` 0 errors, `just comment-lint` and
`just arch-lint` clean. The touched `.vue` files and `api/errors.ts` were
already prettier-unformatted on `develop` (verified against `git show
origin/develop:`), so they are not reformatted here — doing so would bury a
3-line fix in whitespace.
