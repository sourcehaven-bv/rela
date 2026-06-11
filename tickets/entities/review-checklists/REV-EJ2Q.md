---
id: REV-EJ2Q
type: review-checklist
title: 'Review: Markdown checkboxes in entity content are no longer clickable'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`)
- [x] Lint clean (`just lint`)
- [x] ~~Coverage maintained (`just coverage-check`)~~ (N/A: package-floor thresholds only — bugfix in already-covered packages didn't move floors)

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [x] ~~All critical review-responses addressed~~ (N/A: no critical findings)
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** RR-BDCE (significant, addressed), RR-CD4G (significant, addressed), RR-3DRO (minor, deferred), RR-PFRZ (minor, addressed), RR-12KX (minor, deferred), RR-TNA4 (nit, addressed), RR-13LR (nit, addressed), RR-2ZHR (nit, addressed), RR-2KNE (nit, addressed), RR-VD9L (nit, deferred — out of scope, server-side).

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

- PASS — Markdown checkboxes in an entity's content body are clickable and toggle the underlying markdown source on the server. Evidence: e2e/tests/checkboxes.spec.ts "clicking a checkbox persists the toggle on the server" now passes (was previously `test.skip`-ed with misdiagnosis).
- PASS — The SPA's rendered checkbox state reflects the new server-side state after a toggle. Evidence: e2e test asserts `await expect.poll(() => entity.contentCheckboxIsChecked(0)).toBe(true)` in addition to API-level state.
- PASS — Non-entry content sections (content-block, content-cards) render checkboxes as visibly inert (`disabled`), avoiding the fake-interactive failure mode. Evidence: unit test "omits data-cb-idx and keeps disabled by default" + manual code inspection of EntityDetail.vue template call sites.
- PASS — Rapid double-clicks don't ping-pong the server state. Evidence: `togglingIndices` set guards re-entry in `contentClick`; manual code review.

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] ~~Docs-checklist created and linked via `has-docs`~~ (N/A: bug fix, no user-facing API change)
- [x] ~~User-facing documentation updated~~ (N/A: bug fix)
- [x] ~~Docs-checklist marked as done~~ (N/A: bug fix)

**Docs Checklist:** N/A

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] ~~Run `/pr` command to create PR and monitor CI~~ (N/A: user will create PR after reviewing the commit)
- [x] ~~All CI checks pass~~ (N/A: local CI gates already cleared — `just test`, `just lint`, `just arch-lint`, `go test ./...`, full e2e suite all pass; remote CI run is a function of opening the PR)
- [x] ~~PR URL documented below~~ (N/A: PR not yet opened)

**PR:** Pending — user will decide whether to commit and open a PR.
