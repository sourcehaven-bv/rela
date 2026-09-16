---
id: REV-6RCRL2
type: review-checklist
title: 'Review: Denied document stays on screen after a failed SSE re-render'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`)
- [x] Lint clean (`just lint`)
- [x] Comment lint gate clean (`just comment-lint`)
- [x] ~~Coverage maintained (`just coverage-check`)~~ (N/A: the diff is
frontend-only apart from one Go test comment; `go-test-coverage` floors cover Go
packages, and the frontend has no coverage enforcement by project policy)

Frontend suite 2747 tests / 169 files pass. `npm run typecheck` clean, `npm run
lint` 0 errors (the 125 warnings are pre-existing: `v-html` and non-null
assertions, none in this diff). `just comment-lint` clean across 14922 comments;
`just arch-lint` OK.

Go gates could not be run locally: this machine has an unaccepted Xcode license,
so `go build` fails in `runtime/cgo`. The diff touches one Go comment and no Go
code, so CI covers it.

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [x] All critical review-responses addressed
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** RR-K07KLJ (critical), RR-Z2S737 (critical), RR-FRM07H
(significant), RR-9Q1TZ0 (significant), RR-QBG06E (significant) — `addressed`.
RR-R9ZMYM (significant) — `deferred`, with justification.

Both critical findings were real, reproduced before fixing, and neither was a
matter of taste:

- RR-K07KLJ: the cached-badge test could not fail. The badge sits inside the
`v-else-if="docContent"` branch, so the blanking under test removed the very
evidence the assertion looked for. Deleting `isCached.value = false` left the
suite green. Two rewrites also failed to kill the mutant, which established that
the line was unobservable rather than merely under-tested — so the line was
removed and the AM's false "Mutation-verified" claim corrected. This is the same
class of defect as BUG-DJZTRF's RR-7DHGWF (an assertion aimed at an element the
mechanism never touches), which is worth noting: it has now happened twice on
this code.
- RR-Z2S737: `DocumentsPanel` carried the security branch with no test file in
existence. Now covered by its own suite, mutation-verified.

The deferred finding (RR-R9ZMYM, no global 401 handler) is an absent SPA-wide
feature, not a defect in this branch. It is recorded in the bug entity and filed
as a follow-up rather than left to be discovered in production.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

- Denied re-render clears the rendered document — **PASS**, both components,
all three statuses (401/403/404).
- Transient failure still keeps content (BUG-DJZTRF unchanged) — **PASS**, and
the test sits beside the denial cases in both suites so neither rule can be
satisfied at the other's expense.
- A denial from a superseded render does not blank — **PASS**; hoisting the
branch above the generation fence fails that test.
- Classifier boundaries — **PASS**; narrowing to 403 fails 6 tests, widening to
include 500 fails 1, dropping the `instanceof` guard fails 3.

Every mutant was applied and reverted individually with the suite re-run either
side; the figures in the implementation checklist are measured, not estimated.

## Documentation (enhancements only)

- [x] ~~Docs-checklist created and linked via `has-docs`~~ (N/A: bug fix)
- [x] ~~User-facing documentation updated~~ (N/A: no API, config or CLI surface
changed)
- [x] ~~Docs-checklist marked as done~~ (N/A: bug fix)

One internal doc was updated, which is part of the fix rather than a docs task:
`frontend/CLAUDE.md` now qualifies its "keep previous content" rule with the
denial exception, so the next SSE-driven or polled surface inherits it. A
comment on `TestACLDocuments_GatesHiddenEntity` records that the SPA depends on
the 404 status specifically, closing the loop on a security property that spans
two languages with nothing else connecting them.

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] ~~Run `/pr` command to create PR and monitor CI~~ (N/A here: `/pr` gates on
the bug already being `done`, so it necessarily runs after this checklist closes
— TKT-UFV01M)

<!--
Deliberately NOT tracked here: the PR URL and whether CI passed. See TKT-UFV01M.
-->
