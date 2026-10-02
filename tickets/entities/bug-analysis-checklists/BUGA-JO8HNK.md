---
id: BUGA-JO8HNK
type: bug-analysis-checklist
title: 'Analysis: Authored-span property rows overlap and do not edit inline'
started: "2026-10-01"
completed: "2026-10-01"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Reproduction

- [x] Bug reproduced locally (rela-server-postgres on a copy of atlas-dev, TASK-N4W5, 1100px and 1440px viewports)
- [x] Minimal reproduction steps documented (properties section with `span: 2`/`span: 4` fields and `render: input`; Status cell 109px, value column 0px)
- [x] Environment/conditions noted (any viewport where a span cell is narrower than ~320px; the prototype ticket view's `span: 3` fields at 1100px)

## Root Cause

- [x] Immediate cause identified (why1)
- [x] Contributing factors found (why2-3)
- [x] Systemic cause explored (why4-5)

## Fix Planning

- [x] Fix approach determined (RlDetailField becomes a named container and stacks label above value when its own width is narrow; inline controls cap their min-width at the cell)
- [x] Regression test planned (e2e: prototype ticket view at a narrow viewport, every value column has width and stays inside its cell)
- [x] Related areas checked for similar issues (inline editing of the atlas fields is opt-in via `render: input`, TKT-HOIX1; atlas sets it only on status and bijlagen, so that part is a config change in atlas, not a rela defect)
