---
id: BUGA-2OWN0XM
type: bug-analysis-checklist
title: 'Analysis: Flow subject or create type given as an alias skips the visible: check'
started: "2026-11-07"
completed: "2026-11-07"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Reproduction

- [x] Bug reproduced locally (`subject: pers` and `create.type: todo` loaded without a problem in `TestParse_Problems`)
- [x] Minimal reproduction steps documented (bug body)
- [x] Environment/conditions noted (any backend: config load in `internal/flowcfg`, schema with `aliases:`)

## Root Cause

- [x] Immediate cause identified (why1)
- [x] Contributing factors found (why2-3)
- [x] Systemic cause explored (why4-5)

## Fix Planning

- [x] Fix approach determined (reject an alias in `subject` and `create.type`, as `realtimecfg` does)
- [x] Regression test planned (two `TestParse_Problems` cases; aliases added to the test schema)
- [x] Related areas checked for similar issues (`create.type` had the same gap and is fixed; `realtimecfg` already rejects aliases)
