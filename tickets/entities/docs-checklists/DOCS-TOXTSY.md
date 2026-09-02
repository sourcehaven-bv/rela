---
id: DOCS-TOXTSY
type: docs-checklist
title: 'Docs: Gantt perf: header projection + SQL-scoped subtree drill'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Godoc on the new surfaces: `buildGanttSubtree`'s equivalence contract
and decline rules, the subtree-scoped cycle-diagnostic divergence,
`ganttClosureRoundDepth`/`ganttClosureMaxRounds` (why the clamp forces
iteration, why declining beats truncating), `ganttReadVerdict`'s zero-value
guard, `loadGanttType`'s verdict switch.

## Project Documentation

- [x] ~~`docs/data-entry.md`~~ (N/A: no user-facing behaviour or config
change — same wire contract, same semantics, faster)
- [x] ~~`docs/metamodel.md` / `docs/cli-reference.md` / `CLAUDE.md` /
README~~ (N/A: the load-bearing pipeline invariant was already recorded in
CLAUDE.md by TKT-MW28U5; this change operates within it)

## External Docs

- [x] ~~External docs~~ (N/A)
