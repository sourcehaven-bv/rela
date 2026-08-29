---
id: DOCS-HCB4JH
type: docs-checklist
title: 'Docs: extended icon set, generated icon table, icon: none'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Exported symbols have doc comments
- [x] Non-obvious decisions explain WHY, not WHAT
- [x] Nil contracts stated where they matter

`icondefs` carries the package-level rationale: why it is a leaf package (the
generator writes into `dataentryconfig`, so co-locating them would let a broken
generated file make the generator that fixes it unbuildable), why `NoIcon`
travels as a literal rather than `""`, and the two rules for adding an entry.

`spaChromeNames` vs `DerivedNames` each document which coupling direction they
guard and how that coupling fails — the SPA half fails at compile time, the Go
half used to fail silently at render time.

`editDistance` states that it compares bytes, why that is safe (the generator
rejects non-ASCII names), and which direction it fails in.

## Project Documentation

- [x] `docs/data-entry.md` updated — via its source, not directly
- [ ] ~~`docs/metamodel.md`~~ (N/A: icons are data-entry config, not metamodel)
- [ ] ~~`docs/cli-reference.md`~~ (N/A: no CLI surface; the generator is a `just` recipe)
- [ ] ~~`README.md`~~ (N/A: no project-level change)

Three documentation changes, all in
`docs-project/entities/guides/GUIDE-data-entry.md` (the **source**;
`docs/data-entry.md` is generated from it by `just docs`, and hand-editing it
would be reverted on the next docs run):

1. **A generated `### Icon names` section** — all 217 names in a categorised
table with a Name / Glyph / Description column. Inside `<!-- BEGIN generated:
icons -->` markers, written by `just generate-icons`. The Glyph column names the
Lucide component, which is what makes the confusable families (five `Circle*`
glyphs) distinguishable on the page.

2. **`##### Deliberately having no icon`** under Navigation — explains
`icon: none` with the Apple HIG rationale the requester cited, the alignment
behaviour, the distinction from omitting `icon:` entirely, and the
collapsed-sidebar exception.

3. **Rewrote the paragraph that said the name list was "deliberately not
repeated here."** That rationale came from RR-GTOQCF on the predecessor ticket:
a hand-copied list went stale within one release, so it was replaced by a
pointer to the startup error. Generation is precisely the fix that finding
pointed at, so the omission is no longer correct — and leaving it would have
read as a direct contradiction of the table beside it.

Also corrected a kanban paragraph that said omitting `icon:` and writing `icon:
none` "do the same thing". True for columns (they derive no glyph), false for
sidebar items, and stated 250 lines from the sidebar text saying the opposite.
It now says *why* the two coincide for columns and links to the section where
they differ — otherwise a reader reconciling the contradiction would "simplify"
`none` → `""` and silently undo two design-review fixes.

## Contributor-facing docs

- [x] The "adding an icon" instructions updated

`frontend/src/utils/icons.ts` previously told a contributor to edit two lists by
hand. Following that today lands you in a failing drift check. It now says:
append to the Go table, run `just generate-icons`, nothing here changes.

## Discoverability

- [x] The docs are reachable from where an author looks

Both icon sections (kanban columns/swimlanes and navigation items) link to
`#icon-names`. The startup error also points at `docs/data-entry.md#icon-names`
rather than enumerating 217 names inline.

`TestDocsExamplesUseValidNames` asserts every `icon:` value *demonstrated* in
the guide is one the server would accept — the generated table made the
hand-written examples around it look machine-checked, and one of them (`icon:
progress`) had never been valid.
