---
id: DOCS-9XF378O
type: docs-checklist
title: 'Docs: TKT-I3MGCRT'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Comments where logic isn't obvious: why the evaluator uses LoweringSemantics, why the superset filter is sound and matches stored rows, why scanKey includes the where tree
- [x] Function/type docs if public API: godoc on `queryplan.ConditionSuperset`, `LoweredCond.PropCond`, `predicate.Program.ConditionConjuncts`

## Project Documentation

- [x] ~~README updated (if applicable)~~ (N/A: no project-level change)
- [x] ~~CLAUDE.md updated (if new patterns)~~ (N/A: no new convention)
- [x] ~~Help text accurate (if CLI changes)~~ (N/A: no CLI change)

## User-facing Documentation

- [x] `GUIDE-desktop.md` "Conditions": which conditions the database answers, how date conditions are narrowed, unset values, text order; regenerated docs/desktop.md

## External Documentation

- [x] ~~Changelog entry added~~ (N/A: the release changelog is generated from commits)
- [x] ~~API docs updated (if applicable)~~ (N/A: no API change)
