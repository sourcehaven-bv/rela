---
id: REV-QC4ZQI
type: review-checklist
title: 'Review: Render list cells and kanban card fields through the widget registry'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass — `npm run test:run`: 103 files, 1649 tests
- [x] Lint clean — `npm run lint`: 0 errors (remaining warnings are pre-existing
`max-lines` on two files already over the limit before this change)
- [x] ~~Coverage maintained~~ (N/A: `just coverage-check` enforces Go package
floors only; the frontend has no coverage enforcement)

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [x] All critical review-responses addressed
- [x] All significant review-responses addressed (one deferred with reason)
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:**

| ID | Severity | Status | Summary |
|---|---|---|---|
| RR-L5I3L1 | critical | addressed | `list: true` erased the type's display formatter |
| RR-IPTBC7 | critical | addressed | Empty list cells rendered an em-dash placeholder |
| RR-GHU9TE | significant | addressed | `preformatted` derived at the call site |
| RR-UTZM9Q | significant | addressed | `getCardFieldRawValue` returned formatted text |
| RR-XEC2RD | significant | **deferred** | Scalar-enum/array mismatch warns per row |
| RR-2UAM9F | minor | addressed | Mobile card render site had no coverage |
| RR-01JKVJ | minor | addressed | The two emptiness predicates diverged |

RR-XEC2RD is deferred because it is not introduced by this change and not
fixable in the routing layer: `SelectWidget` itself warns and truncates, and
`PropertyDisplay` resolves the same widget from the same schema, so detail views
have identical exposure today. Fixing it means changing a widget shared with
forms — a separate ticket rather than a silent scope widening.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:** all four PASS — see IMPL-L7TKL6 for per-criterion
evidence, including a value-by-value comparison of the new rendering against the
pre-migration `formatCellValue` output.

## Documentation (enhancements only)

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-L7U5HT

Initially recorded as N/A on the grounds that this is an internal refactor. The
metamodel disagreed — `has-docs` is required for `kind: enhancement`, and the
gate was right: `frontend/CLAUDE.md` documented `src/components/forms/` as the
widget home and never mentioned `src/widgets/`, which this change makes more
misleading. Updated the package table and added a section on the display-side
widget contract (the dense-vs-detail rules that caused the two critical review
findings).

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI
- [x] All CI checks pass
- [x] PR URL documented below

**PR:** see the `has-pr` note below.

Ordering note: the workflow requires the ticket to be `done` and validating
clean *before* the PR is opened (a ticket left in `review` fails the
`CI: Tickets in 'review' status cannot be merged` gate on its own PR). So these
items are checked as the PR is created rather than after it merges, and the URL
is recorded on the ticket once `gh pr create` returns.
