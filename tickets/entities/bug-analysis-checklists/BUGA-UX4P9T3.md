---
id: BUGA-UX4P9T3
type: bug-analysis-checklist
title: 'Analysis: Phone list titles, laptop table, banner placement and phone Overview header'
started: "2026-11-22"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Reproduction

- [x] Bug reproduced locally (a local server on a copy of the saas-dev config, browser pane at 375px and 1024px)
- [x] Minimal reproduction steps documented (an invoice list with two kept columns at 375px; any five-column list at 1024px; the Overview with a notice at 375px)
- [x] Environment/conditions noted (phone and laptop widths, read-only workspace)

## Root Cause

- [x] Immediate cause identified (why1)
- [x] Contributing factors found (why2-3)
- [x] Systemic cause explored (why4-5)

## Fix Planning

- [x] Fix approach determined (preferred and minimum column widths in columnSizing.ts; widths by property type; a title floor that wraps kept cells; breakpoint 640px; banner above the header; dashboard title through PageHeaderContent)
- [x] Regression test planned (vitest for columnSizing and tableColumns, e2e mobile-layout and list)
- [x] Related areas checked for similar issues (stack mode shares the breakpoint and resets the new flex sizes; entity pages already used the header band)
