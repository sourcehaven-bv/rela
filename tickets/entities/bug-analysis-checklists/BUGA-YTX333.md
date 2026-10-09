---
id: BUGA-YTX333
type: bug-analysis-checklist
title: 'Analysis: dataentry 500 responses send the internal error text to the client'
started: "2026-10-09"
completed: "2026-10-09"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Reproduction

- [x] Bug reproduced locally (TestNoInternalErrorDetail failed on 21 call sites before the fix)
- [x] Minimal reproduction steps documented (any store failure on e.g. DELETE /api/v1/{type}/{id} returned the error text in detail)
- [x] Environment/conditions noted (all backends; the text differs per backend, pgstore names schemas and tables)

## Root Cause

- [x] Immediate cause identified (why1)
- [x] Contributing factors found (why2-3)
- [x] Systemic cause explored (why4-5)

## Fix Planning

- [x] Fix approach determined
- [x] Regression test planned (AST guard over the package plus a helper contract test)
- [x] Related areas checked for similar issues (MCP is TKT-OQ7MDF; 500s with other variable names in dataentry already send a generic detail)
