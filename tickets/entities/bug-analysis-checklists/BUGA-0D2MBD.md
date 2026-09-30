---
id: BUGA-0D2MBD
type: bug-analysis-checklist
title: 'Analysis: View table cells render every typed value as a badge'
started: "2026-09-30"
completed: "2026-09-30"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Reproduction

- [x] Bug reproduced locally (atlas demo, project PROJ-30ZY, "Taken" table)
- [x] Minimal reproduction steps documented (bug Report section)
- [x] Environment/conditions noted (any `display: table` section with a non-enum typed column)

## Root Cause

- [x] Immediate cause identified (why1)
- [x] Contributing factors found (why2-3)
- [x] Systemic cause explored (why4-5)

## Fix Planning

- [x] Fix approach determined (route each cell through the server-resolved widget, as nestedCellsFor does)
- [x] Regression test planned (EntityDetail.table.test.ts)
- [x] Related areas checked for similar issues (grouped and ungrouped tables both fixed; cards/list fields still use viewFieldRoutingHint, noted in prevention)
