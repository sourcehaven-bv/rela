---
id: BUGA-21NIRGZ
type: bug-analysis-checklist
title: 'Analysis: View section tables on phones, list band and title link colour'
started: "2026-11-21"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Reproduction

- [x] Bug reproduced locally (harbourmaster account page at phone width: the three sections scroll sideways; a list shows a 20px plus 16px band; the title is rgb(242,242,240) while the accent is #82b5fa)
- [x] Minimal reproduction steps documented (open an entity with a display table view section at 375px; open any list with a link title column)
- [x] Environment/conditions noted (portal user, dark theme; the band and colour also show on desktop)

## Root Cause

- [x] Immediate cause identified (why1)
- [x] Contributing factors found (why2-3)
- [x] Systemic cause explored (why4-5)

## Fix Planning

- [x] Fix approach determined (a container query stacks the section table; primary allowed on table section columns and sent on the wire; the list table cancels the pane padding; the title link gets the accent colour)
- [x] Regression test planned (sectionTable.test.ts, SectionTableRowCells.test.ts, EntityList.titleLink.test.ts, TestViewColumnsToV1_Primary, the e2e mobile spec)
- [x] Related areas checked for similar issues (dashboard table cards already stack; long values without spaces overflowed and now wrap)
