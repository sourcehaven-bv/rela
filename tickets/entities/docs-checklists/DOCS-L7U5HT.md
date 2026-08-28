---
id: DOCS-L7U5HT
type: docs-checklist
title: 'Documentation: widget-based display rendering'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Comments where logic isn't obvious

The three non-obvious decisions each carry a comment explaining *why*, because
each is the kind a future reader would "simplify" back into a bug:
`densePropertyRoutingHint`'s list-after-type ordering (with the regression it
caused), the `boolean → text` / `file → text` exceptions, and `isDenseEmpty`'s
dense-vs-detail contract split. `EntityList`'s `resolveCell` memo documents why
a WeakMap keyed on a mutable entity is safe (copy-on-write) and what would break
that assumption.

- [x] Function/type docs if public API

`DenseRoutingHint`, `densePropertyRoutingHint`, and `isDenseEmpty` are the new
exported surface; all three have doc comments.

## Project Documentation

- [x] ~~README updated~~ (N/A: no project-level change — no new command, config
key, or install step)
- [x] CLAUDE.md updated (if new patterns)

`frontend/CLAUDE.md` gained a "Widgets render property values everywhere,
including read-only" section covering the two routing entry points, the four
dense-surface rules, and the `preformatted` contract. Also corrected the package
table, which listed `src/components/forms/` as the widget home and omitted
`src/widgets/` entirely — stale before this change and more misleading after it.

- [x] ~~Help text accurate~~ (N/A: no CLI change)

## External Documentation

- [x] ~~Changelog entry added~~ (N/A: the repo has no CHANGELOG.md — verified,
not assumed)
- [x] ~~API docs updated~~ (N/A: no HTTP API, wire-schema, or metamodel change.
`docs/data-entry.md` documents configuration, and no config surface changed —
`widget:` on `ListColumn`/`KanbanCardField` was explicitly left out of scope)

## Note on user-visible behaviour

This is mostly an internal refactor, but the kanban half is a user-visible bug
fix: cards previously showed raw ISO datetimes and `"true"`, and silently
dropped any field whose value was `false` or `0`. That is captured in the ticket
and the commit message rather than in end-user docs, since there is no document
describing what a kanban card renders for a given property type.
