---
id: DOCS-NQS6CB
type: docs-checklist
title: 'Docs: display: nested view section'
status: done
---

## Code docs

- [x] Godoc on every new exported and non-trivial unexported symbol:
`dataentryconfig.DisplayNested`, `ViewSection.Children`, `viewResult.Parents`,
`buildNestedTree`, `planNestedRows`, `nestedRow`, `sectionTreeNodeToV1`,
`buildNestedEntityData`, `buildSectionRow`, `fillPropertyCell`,
`validateNestedSection`, `nestedChildDef`, `v1.ViewTreeNode`,
`ViewSection.Tree`/`.Truncated`, and the two budget constants.
- [x] Comments state WHY, not what. The load-bearing ones: why `Collections`
cannot answer parent attribution, why `viewResult.Parents` is explicitly NOT an
authorization boundary, why selection precedes relation-column resolution, why
`recursive: true` is refused, and why the row builder was extracted rather than
copied.
- [x] `just comment-lint` gate clean; `just comment-report` introduces no new
advisory findings in the changed files (the two in `responses.go` are at lines
300 and 781, pre-existing).

## Project docs

- [x] `docs/data-entry.md` — `nested` added to the Display Modes table, plus a
`#### nested — parent→child trees` subsection with a worked config example and
the behaviours an operator needs: that nesting comes from the traverse rules
rather than `children:` alone, that `columns:` apply at both levels, that
children are unsorted for now, that `recursive:` is refused, that large trees
are capped with `hasMoreChildren`/`truncated`, and that rows carry no markdown
body.
- [x] `children` added to the section fields reference table, and `columns`
updated to say it applies to `table` **and** `nested`.
- [x] ~~`docs/metamodel.md`~~ (N/A: no metamodel change)
- [x] ~~`docs/cli-reference.md`~~ (N/A: no CLI change)
- [x] ~~`CLAUDE.md`~~ (N/A: the feature follows existing documented rules —
gate-before-fold, content-free collection reads — rather than adding one)

## External docs

- [x] ~~README~~ (N/A: not a project-level change)
- [x] ~~Changelog~~ (N/A: no changelog is maintained in-tree)

## Stale docs found while working (not fixed here)

Both deserve their own ticket rather than riding along in this diff:

1. **`docs/data-entry.md` § "Entity Views"** still documents
`entity_views:` / `detail_view:` as the current way to bind a view to a type.
`rela migrate` reports `views-by-entity-type: Re-key views: by entity type,
remove detail_view and entity_views`, so that section describes a migrated-away
syntax. This actively misled the planning for this ticket.
2. **`docs/customisation.md:206`** says `<rela-slot>` is "Not yet emitted",
but it is live in `NextActionCard.vue:51` and `StatusBar.vue:156`.
