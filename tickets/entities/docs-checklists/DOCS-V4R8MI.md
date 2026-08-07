---
id: DOCS-V4R8MI
type: docs-checklist
title: 'Docs: Render admin-authored header/footer markdown on kanban boards'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Godoc / doc comments on new exported symbols

`dataentryconfig.Kanban` gained a type doc comment explaining what `Header` and
`Footer` are and — importantly — why there is deliberately no `Description`
fallback, so the next reader does not "restore symmetry" with `List` by adding
one.

- [x] Non-obvious decisions explained at the point of the decision

Three comments carry reasoning that would otherwise be lost:

- `viewHeaderMarkdown` JSDoc — why the `description` alias is a runtime opt-in
flag rather than a type constraint (types erase; configs come off the wire).
- `.kanban-board` CSS — why `overflow-x: auto` also clips vertically per spec,
why `overflow-y: visible` cannot opt out, and what would be clipped in future.
- `KanbanView.test.ts` AC3 test — what the sanitization assertion does and does
not prove, and why (happy-dom DOM defects, verified against jsdom).

## Project Documentation

- [x] `docs/data-entry.md` updated

Added `header`/`footer` rows to the Kanban Fields table and a "Header and footer
info regions" subsection that cross-references the list section rather than
duplicating it, documents that the regions sit outside the horizontal scroll
area, and states explicitly that kanban has no `description` fallback.

- [x] ~~`docs/metamodel.md`~~ (N/A: data-entry config, not metamodel schema)
- [x] ~~`docs/cli-reference.md`~~ (N/A: no CLI surface)
- [x] ~~`CLAUDE.md`~~ (N/A: no new pattern — the shared stylesheet follows the
existing `frontend/src/styles/` convention set by `back-button.css`)
- [x] ~~`README.md`~~ (N/A: not a project-level change)

## Example Configuration

- [x] Working example in a real project config

`tickets/data-entry.yaml` `idea_board` now sets both `header` and `footer`,
mirroring what TKT-H7E611 did for the `all_ideas` list. This doubles as the
manual-verification fixture and gives operators a copy-paste starting point.

## External Documentation

- [x] ~~Release notes / migration guide~~ (N/A: purely additive on the config
surface — omitted `header`/`footer` keys behave exactly as before, so no
existing `data-entry.yaml` needs changing)

One behavior change is worth noting for reviewers even though it needs no
migration: the board's horizontal scroll moved from the page wrapper onto the
board containers, so on a wide board the page title and filter bar no longer
scroll sideways. That is a fix to a pre-existing quirk, affects every board, and
is documented in the ticket and in the CSS comment.
