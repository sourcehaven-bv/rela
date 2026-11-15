---
id: DOCS-QL2BJCH
type: docs-checklist
title: 'Docs: list condition pushdown (TKT-I39Y7N4)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Comments where logic isn't obvious (scalar `~=` on off-type values, denied negated traversals, EntityAttributes completeness, per-rule nav counts)
- [x] Function/type docs if public API (queryplan.LowerCondition, predicate.Program.ConditionConjunction, store.PropPredicate.Scalar)

## Project Documentation

- [x] ~~README updated (if applicable)~~ (N/A: no README change)
- [x] CLAUDE.md updated (if new patterns) (.claude/rules/collection-reads.md names LowerCondition and the visibility rule)
- [x] ~~Help text accurate (if CLI changes)~~ (N/A: no CLI change)

## External Documentation

- [x] ~~Changelog entry added~~ (N/A: the repo has no changelog)
- [x] ~~API docs updated (if applicable)~~ (N/A: rows and totals are unchanged; no API surface change)
