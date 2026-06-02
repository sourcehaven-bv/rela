---
id: REV-U0N6
type: review-checklist
title: 'Review: Bundle Font Awesome locally so EasyMDE doesn''t fetch it from a CDN'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass — `frontend/`: 733 vitest tests green; `e2e/`: 192 Playwright tests green (1 unrelated skip)
- [x] Lint clean — `npm run lint` in frontend (0 errors, only pre-existing warnings in stress/ tests unrelated to this ticket); `eslint` on touched e2e files clean
- [x] ~~Coverage maintained~~ (N/A: `just coverage-check` is for Go code; this ticket is frontend-only configuration + e2e — frontend coverage is the per-file ratchet which neither the touched component nor the spec/page-object change since no new branching logic was added)

## Code Review

- [x] Run `/code-review` command — cranky-code-reviewer ran against the diff
- [x] All critical review-responses addressed — RR-3AMR, RR-7ZB8 both `addressed`
- [x] All significant review-responses addressed — RR-TDL4, RR-1FL4, RR-EHRX all `addressed`
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:**

- Critical: RR-3AMR (addressed), RR-7ZB8 (addressed)
- Significant: RR-TDL4 (addressed), RR-1FL4 (addressed), RR-EHRX (addressed)
- Minor: RR-LSGW (addressed), RR-VUKJ (addressed)
- Nit: RR-11CM (addressed), RR-UIR6 (addressed), RR-W8MF (addressed), RR-6DWU (addressed), RR-PBV5 (wont-fix with reason)

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

- **AC1 (no maxcdn fetch): PASS** — the appPage fixture's same-origin guard runs against the full 192-test suite and would fail any test where the SPA fetches `maxcdn.bootstrapcdn.com` (or any other off-origin host). All tests green.
- **AC2 (icons render): PASS** — `markdown editor bundles Font Awesome (no CDN fetch)` test asserts `getComputedStyle('.editor-toolbar .fa-bold', '::before').fontFamily` contains `fontawesome`. Test green.
- **AC3 (same-origin assets): PASS** — same fixture guard subsumes this; off-origin assets of any kind fail every affected test, not just font extensions.
- **AC4 (offline): PASS** — the fixture-level same-origin assertion is logically equivalent to "the SPA needs nothing beyond the binary's origin." If the bundle ever required an external host, every test using `appPage` would fail at teardown.

## Documentation (enhancements only)

- [x] ~~Docs-checklist created and linked via `has-docs`~~ (N/A: no user-facing docs affected, as established in planning — `data-entry-ui` and `FEAT-021` documentation already implies self-contained-binary; this fix makes the implementation match the documented story)

## Final Checks

- [x] Commit message will explain the why (CDN coupling violated self-contained-binary deployment story), not just the what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use — every future e2e test now inherits the regression guard automatically

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI
- [x] All CI checks pass
- [x] PR URL documented below

**PR:** https://github.com/sourcehaven-bv/rela/pull/882
