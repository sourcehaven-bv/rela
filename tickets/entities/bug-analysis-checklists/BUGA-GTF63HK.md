---
id: BUGA-GTF63HK
type: bug-analysis-checklist
title: 'Analysis: Regex search on an enum property finds nothing'
started: "2026-10-30"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Reproduction

- [x] Bug reproduced locally (`TestMatchEnumEdgeCases` regex cases and `TestMatchCustomType` fail on develop: `operator "=~" not supported for enum property`)
- [x] Minimal reproduction steps documented (bug body: `status=~dra` against an enum value `draft`)
- [x] Environment/conditions noted (any backend; the data-entry `_search` and query cards run property filters through `filter.MatchAll` in Go)

## Root Cause

- [x] Immediate cause identified (why1)
- [x] Contributing factors found (why2-3)
- [x] Systemic cause explored (why4-5)

## Fix Planning

- [x] Fix approach determined (allow `=~` on enum and custom types in `validateOperatorForType`; in `matchEnum` match the regex before the allowed-value check)
- [x] Regression test planned (filter unit tests for inline enum and custom type)
- [x] Related areas checked for similar issues (`predicatefns.FromFilter` already transpiles `=~` for any type; list pushdown only pushes equality; `matchesPropertyFilters` swallowing errors is a separate defect, noted in the bug body)
