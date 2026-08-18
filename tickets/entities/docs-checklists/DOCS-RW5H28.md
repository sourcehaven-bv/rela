---
id: DOCS-RW5H28
type: docs-checklist
title: 'Docs: View section fields render as display by default; opt in to inline edit with `render: input`'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Godoc / comments on new exported symbols
- [x] Non-obvious decisions explained at the point of the decision
- [x] "Keep in sync" hazards documented where they exist

`ResolveFieldRender`, `RenderDisplay`/`RenderInput`, and the `Render` fields on
`ViewSection` / `ViewSectionField` all carry godoc. The load-bearing hazards are
documented where a future editor will actually hit them:

- `v1.SectionField` and `dataentry.SectionFieldData` each carry a KEEP IN SYNC comment naming
the other and listing all four unnamed-conversion sites.
- The `writable` conjunct in `SectionEditForm.vue` explains why it is a conjunction, why it is
applied at that site only, and why `render` must not go through
`isFieldWritable`'s `fieldReadonly` parameter (RR-PGGRBD).
- `sectionShouldRouteToInlineEdit` explains why its predicate must match `widgetRows`.
- `sectionDisplayModesRenderingFields` documents why `content` is deliberately absent despite
the builder populating fields for it, and warns against "fixing" it (RR-VBJ91V).
- `entryDisplayValue` explains the staleness it prevents and why the default flip made it
reachable (RR-GLK4UY).
- `isLongValue` states exactly what is and isn't shared with `PropertyDisplay.isLong`
(RR-1SNYI1).

## Project Documentation

- [x] `docs/data-entry.md` updated
- [x] Breaking change documented
- [x] ~~docs/metamodel.md~~ (N/A: `data-entry.yaml` config, not metamodel)
- [x] ~~docs/cli-reference.md~~ (N/A: no command changes)
- [x] ~~CLAUDE.md~~ (N/A: no new architectural pattern; the change follows existing ones)

`docs/data-entry.md` gained:
- a `render` row in the section options table and a new per-field options table
- a "Field Render Modes" section with before/after YAML for both field- and section-level use
- the rule that `render: input` cannot upgrade an ACL-read-only field
- which display modes honour `render`, and that the side panel ignores it
- a breaking-change blockquote naming the migration action
- a correction to the stale line describing views as "read-only detail pages"

## External Documentation

- [x] ~~Changelog / release notes~~ (N/A: this repo has no `CHANGELOG.md`, upgrade-notes, or
release-notes file — verified with `find`. Creating one solely for this ticket
would establish a surface the project has never maintained. See RR-H39SEJ.)
- [x] ~~README~~ (N/A: not a project-level change)

## Migration Guidance

- [x] Migration path documented for existing configs
- [x] In-repo configs migrated as a worked example

Operators add `render: input` to fields (or once per section) they want to keep
editable. The in-repo migration demonstrates the recommended granularity:
section-level `render: input` on each `properties`/`cards`/`list` section — 23
sections in `tickets/data-entry.yaml`, 5 in `prototypes/data-entry/project`, and
none on `table`/`content` sections where it would be inert.
