---
id: REV-C62E3
type: review-checklist
title: 'Review: Add Edit button to data-entry document view'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] `go test ./...` — all packages pass.
- [x] `just lint` — `0 issues.` from golangci-lint.
- [x] `npm run typecheck` (frontend) — no errors.
- [x] `npm run test:run` (frontend) — 477 tests pass across 23 files.
- [x] E2E `document-edit-button.spec.ts` — 4/4 pass.

## Code Review

- [x] Cranky code review run via the cranky-code-reviewer agent. Found 9
issues; 8 addressed, 1 deferred (pre-existing TS type drift on DocumentConfig).

**Review responses:**

Significant (all addressed):
- RR-4AXJR — bare `edit:` YAML semantic documented in struct comment + docs.
- RR-FXOLY — `TestValidateConfig_DocumentsEditBothEmpty` pins the
both-empty contract.
- RR-1F8AO — `suggestForm()` typo suggestion added; matches the wording
of `list.edit_form` errors. Test added.

Minor (all addressed):
- RR-ZMK1U — hoisted hardcoded literals in e2e to local consts.
- RR-XJ1G1 — `DocumentPage.editButton` scoped to `.header-right`.
- RR-2OGRV — added `script:` + `edit:` test row.
- RR-M2HUU — added dedicated `feature_edit` form (`mode: edit`) to fixture.

Nits:
- RR-LWO4N (addressed) — dropped dead defensive check, replaced with
non-null assertion + comment.
- RR-R0VC2 (deferred) — pre-existing TS type drift on `DocumentConfig`,
out of scope for this ticket.

Design-review responses (10 from `/design-review`):
- 2 significant — addressed (RR-6NIDO, RR-SFG9K).
- 6 minor/nit addressed or obviated by the redesign.
- RR-WD6MB, RR-VFPYX — explicitly accepted as wont-fix with reasons.
- RR-4P8I0 — keyboard shortcut deferred (out of scope).

## Acceptance Verification

- [x] AC1 (button visible when configured): PASS — e2e
`renders Edit button when edit block is configured`.
- [x] AC2 (clicking navigates with `return_to`): PASS — e2e
`Edit button click navigates to the form with return_to set to the document
path`.
- [x] AC3 (saving returns to document): PASS — e2e
`saving the form returns to the document URL`.
- [x] AC4 (validation rejects unknown form): PASS — Go test
`edit.form references unknown form` + suggestion-aware variant.
- [x] AC5 (validation rejects empty form / empty label): PASS —
three Go test cases.

## Quality

- [x] All critical/significant review responses resolved.
- [x] Plan compliance verified — implementation matches the documented
Approach section.
- [x] No new patterns introduced — follows existing
`list.edit_form` / `kanban.edit_form` validation pattern and existing
`buildReturnTo` / `readReturnTo` flow.
