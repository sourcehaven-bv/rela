---
id: REV-UK2DUT
type: review-checklist
title: Review
status: done
---

## Automated Checks

- [x] All tests pass (`just test`)
- [x] Lint clean (`just lint`)
- [x] Comment lint gate clean (`just comment-lint`)
- [x] Coverage maintained (`just coverage-check`)

Go: every package passes except `cmd/rela-desktop`, which fails to BUILD in this
environment on an Xcode-license/cgo error. Confirmed pre-existing by stashing
the change and reproducing the identical failure. The same cause makes `just
lint` and `just plimsoll` report typecheck errors for that one package; both are
clean otherwise.

Coverage: package floors all satisfied, total 79.6%.

Frontend: 2665 tests, 163 files. ESLint back to 0 errors (one `prefer-const` was
introduced and fixed); the 124 warnings are pre-existing.

E2E: 298 passed, 8 skipped. The editor block was run three times over to confirm
a race I had introduced in the page object was gone.

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [x] All critical review-responses addressed
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:**

Nine findings, no critical. All addressed.

| ID | Severity | Finding |
|----|----------|---------|
| RR-TRIRRS | significant | Reference button destroyed the user's selection |
| RR-7QE2I8 | significant | Withdrawal left the guard disarmed, and its justifying comment was wrong |
| RR-4GELDN | significant | Withdrawal consumed its record before checking it could act |
| RR-0FRDLA | minor | Pending trigger span survived into a different document |
| RR-10GMGV | minor | Stylesheet error path was live in the suite, asserted by nothing |
| RR-AV8B8E | minor | Icon colour test asserted a weaker property than its name |
| RR-P4FYPI | minor | CSP-violation e2e test filtered by a hardcoded seed entity ID |
| RR-E0JRND | nit | e2e files reformatted wholesale by a stray Prettier run |
| RR-FHJZC3 | nit | Stale reference to a deleted module in a comment |

All three significant findings were in the same thirty lines —
`_promptForRef`/`_withdrawPromptedRef`, which existed only to fix a nit
(RR-LF36QW) from the design review. That is the lesson worth keeping: the
cheapest-looking fix in the change carried every blocking defect in it, because
it was written quickly against a nit rather than designed.

RR-LF36QW's resolution was rewritten to say it is superseded, since it recorded
as "addressed" a fix that reintroduced the same defect by another route.

One disputed claim was resolved by measurement rather than argument. The review
disproved a comment of mine with a probe showing the document fully reverts
after a withdrawal; a structural probe showed a leftover EMPTY paragraph that
`textContent` cannot see. So my comment was closer to right than its disproof —
and both were reasoning where a measurement was needed, which is exactly why the
code now decides it by comparing serializations instead of by either argument.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

1. *An app needs no change.* PASS — 34 tests in `relaEditor.test.ts` against the
real editor, covering the whole six-item contract plus teardown and reconnect.
2. *The two editors cannot serialize a body differently.* PASS —
`editorPreset.ts` now holds the plugin set and serializer options for both;
`editorPreset.test.ts` pins its output and greps both editor sources to fail if
either rebuilds the stack locally. Strengthened from the import-graph argument
the plan originally offered (RR-3KSGIS).
3. *Works under the real app CSP in a real browser.* PASS — 8 e2e tests in the
sandboxed iframe, including zero CSP violations over a window scoped to the
editor interaction (RR-P4FYPI).
4. *No webfont ships.* PASS — the build fails on `@font-face`, an inlined font
data URI, or a font `url()` in either artifact, verified by deliberately adding
one. Plus the 404 on the old path in both Go and e2e.
5. *`.value` is churn-free.* PASS — and both halves are now pinned: an unedited
body byte-identical, an edited one honestly reserialized (RR-ZW2IEO).

## Documentation (enhancements only)

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-5Q6CXV

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] ~~Run `/pr` command to create PR and monitor CI~~ (N/A: the PR post-dates
      this checklist — `/pr` gates on the ticket already being `done` and
      validating clean, so this item cannot be satisfied before it runs. See
      TKT-UFV01M.)
