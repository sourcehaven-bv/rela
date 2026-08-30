---
id: DOCS-WP2LNF
type: docs-checklist
title: 'Docs: Ctrl/Cmd-click should open data-entry rows and cards in a new browser tab'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] New/changed functions have doc comments
- [x] Complex logic explained with WHY comments
- [x] Nil contracts documented where relevant

`openIntent.ts` documents why the helper is named `shouldDeferToBrowser` rather
than `wantsNewTab` (it also covers `defaultPrevented`, which means the opposite
of "open a tab"), and why `safeInternalHref` rejects a following slash OR
backslash. `entityTarget` in each component states that it is the single source
of truth for both the link and any push, and what silently breaks otherwise. Nil
contracts are stated where a helper returns `undefined` (RelationCards' uncached
entry, IssuesTable's entity-less rows, EntityDetail's unresolvable target). The
Kanban card carries a comment saying why it deliberately has NO
`draggable="false"`, so the next reader does not "fix" it and break reordering.

## Project Documentation

- [x] `docs/data-entry.md` updated
- [x] ~~docs/metamodel.md, docs/cli-reference.md, README.md~~ (N/A: no
metamodel, command or project-level surface change)
- [x] ~~CLAUDE.md~~ (N/A: no new cross-cutting convention; the change follows the
existing `DashboardView` precedent rather than introducing a pattern)

New "Opening things in a new tab" section under Overview: which gestures work,
that a new tab keeps the list context so prev/next still walks the same result
set, and the accepted trade-off that a list row is a link across its full width
so text selection inside a row is not possible.

## External Documentation

- [x] ~~API docs~~ (N/A: no API surface change — this is SPA markup only)
- [x] ~~Migration notes~~ (N/A: no config, schema or data change; nothing for an
operator to do)

## Test Documentation

- [x] Non-obvious test setups explained

`test/setup.ts` explains why the global RouterLink stub emits a real `href` (the
previous `<a><slot/></a>` dropped `to`, making every href assertion vacuous) and
warns that its `URLSearchParams` encoding only APPROXIMATES vue-router, so
exact-encoding assertions belong in e2e. The new e2e fixture section states that
the entity-detail table display previously had no coverage at all, which is how
an unconditional `@click.prevent` survived on those anchors.
