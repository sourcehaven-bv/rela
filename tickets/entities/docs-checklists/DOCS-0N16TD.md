---
id: DOCS-0N16TD
type: docs-checklist
title: 'Docs: Traverse condition: predicate expressions on view traverse steps (TKT-0N16TD)'
started: "2026-10-15"
completed: "2026-10-15"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Comments where logic isn't obvious (`ViewTraverse.Condition`, `CompileForCollection`, `compileTraverseConditions`, `traverseFilter`, `TraverseConditionID`)
- [x] Function/type docs if public API (`TraverseTargetTypes`, `TraverseConditionID`, `SplitTraverseConditionID`, `Evaluator.CompileForCollection`)

## Project Documentation

- [x] ~~README updated (if applicable)~~ (N/A)
- [x] ~~CLAUDE.md updated (if new patterns)~~ (N/A: follows the list `condition:` rule in `.claude/rules/predicate.md`)
- [x] ~~Help text accurate (if CLI changes)~~ (N/A: no CLI change)

## External Documentation

- [x] ~~Changelog entry added~~ (N/A: the repo has no changelog)
- [x] API docs updated (if applicable) (GUIDE-data-entry.md: traverse field table, "Traverse conditions (`condition:`)" with the two examples, the `where:` load error replacing the fail-open note. Regenerated into docs/data-entry.md with scripts/generate-docs.sh)
