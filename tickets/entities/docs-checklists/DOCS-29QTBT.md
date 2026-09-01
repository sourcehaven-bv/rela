---
id: DOCS-29QTBT
type: docs-checklist
title: 'Docs: Hierarchical Gantt view for data-entry'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Godoc on all new exported types/functions (`Gantt`, `GanttSource`,
`GanttNode`/`GanttResponse` wire types, `NormalizeGantts`)
- [x] The security-load-bearing pipeline documented ON the type
(`ganttHandler` doc: gate → redact once → fold → cap, with the tickets and the
failure mode named)
- [x] Non-obvious choices commented at the site (post-redact `where:`,
match-error direction, `entityTimeValue`-not-`GetString`, budget rule, iterative
fold rationale, local-calendar todayDay)

## Project Documentation

- [x] `docs/data-entry.md` — full `## Gantts` section (config example,
recursive-containment + drill-down explanation, planned-vs-rolled and the two
breach textures, field tables, policies, validation list, limits) — authored in
`docs-project/entities/guides/GUIDE-data-entry.md` and regenerated via `just
docs`
- [x] `CLAUDE.md` — new "Aggregates computed over the graph gate BEFORE they
fold" rule recording the invariant and its pinning tests
- [x] ~~`docs/metamodel.md`~~ (N/A: no metamodel change)
- [x] ~~`docs/cli-reference.md`~~ (N/A: no CLI change)

## External Docs

- [x] ~~README~~ (N/A: feature-level, covered by the data-entry guide)
