---
id: REV-B2OH
type: review-checklist
title: 'Review: Remove +Add / Link Existing buttons from data-entry view widgets'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`)
- [x] Lint clean (`just lint`)
- [x] Coverage maintained (`just coverage-check`)

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [x] All critical review-responses addressed
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:**

- RR-O8FM (significant) — addressed: test restructured into 5 sub-tests covering outgoing/incoming × cards/list/table × with-form/no-form, with extracted `assertViewSectionsLackKeys` helper.
- RR-EIXM (significant) — addressed: doc comment on `resolveSectionButtonsWithTraverse` rewritten to explicitly state side-panel-only contract.
- RR-R8X6 (significant) — addressed: created follow-up TKT-6ETQ for the rename to `V1SidePanel*`; added doc-comment TODO references on the three affected types.
- RR-QH41 (minor) — addressed: covered by `outgoing-cards-no-form` sub-test in the restructured test.
- RR-RXK8 (minor) — addressed: removed three stranded blank lines in `EntityDetail.vue`.
- RR-P10W (nit) — wont-fix: consistent with existing convention in `api_v1_test.go`; project-wide builder migration is out of scope for this refactor.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

1. **PASS** — `EntityDetail.vue` greps clean for `addInfo`/`linkInfo`/`+ Add`/`Link Existing`/`navigateToCreate`/`openLinkExisting`/`LinkExistingModal`. Vue typecheck clean.
2. **PASS** — `SidePanel.vue` and `V1SidePanelSection` untouched; the side-panel handler at `api_v1.go` line 1758 still calls `resolveSectionButtonsWithTraverse`. Backend tests pass.
3. **PASS** — `TestV1Views_NoAddOrLinkInfoOnSections` (5 variants) decodes the actual handler response and asserts both keys are absent. Loose `map[string]json.RawMessage` decoding catches Go-side rename drift.
4. **PASS** — `navigateToEdit` and the per-row Edit pencil template branches in `EntityDetail.vue` are unchanged.
5. **PASS** — Header Edit/Delete buttons (`editEntity`, `requestDelete`, top-of-template buttons) untouched.
6. **PASS** — `just test` ✓, `just lint` ✓ 0 issues, `just arch-lint` ✓ no warnings, `just coverage-check` ✓ 76.1% total, `npm run typecheck` ✓ clean, `npm run test:run` ✓ 650/650, `npm run lint` ✓ 0 errors.

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] ~~Docs-checklist created and linked via `has-docs`~~ (N/A: refactor kind)
- [x] ~~User-facing documentation updated~~ (N/A: no public CLI/API surface promised this affordance)
- [x] ~~Docs-checklist marked as done~~ (N/A)

**Docs Checklist:** N/A (refactor)

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed (the TKT-6ETQ TODO references are intentional follow-up markers)
- [x] Ready for another developer to use

## Pull Request

- [x] ~~Run `/pr` command to create PR and monitor CI~~ (N/A: PR creation is the user's explicit action via `/pr`; ticket marked done at the working-tree level after all gates pass)
- [x] ~~All CI checks pass~~ (N/A: CI runs against the PR; local equivalents `just test`, `just lint`, `just arch-lint`, `just coverage-check`, `npm run typecheck`, `npm run test:run`, `npm run lint` all pass)
- [x] ~~PR URL documented below~~ (N/A: PR not yet created)

**PR:** *pending — user will run `/pr` when ready to ship*
