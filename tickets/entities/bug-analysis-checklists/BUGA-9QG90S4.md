---
id: BUGA-9QG90S4
type: bug-analysis-checklist
title: 'Analysis: File-store conflict-block refusal answers HTTP clients with a generic error'
started: "2026-11-06"
completed: "2026-11-06"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Reproduction

- [x] Bug reproduced locally (TestWrite_ConflictBlockBodyIs422: relation content in a unified PATCH answered 500 `relation_write_failed`; entity create and PATCH answered 422 `validation_failed`)
- [x] Minimal reproduction steps documented (file store; body with a complete conflict block inside a fenced code block; POST or PATCH it)
- [x] Environment/conditions noted (file store only; the memory, SQLite and Postgres stores accept the content)

## Root Cause

- [x] Immediate cause identified (why1)
- [x] Contributing factors found (why2-3)
- [x] Systemic cause explored (why4-5)

## Fix Planning

- [x] Fix approach determined (`store.ErrConflictBlock` sentinel; fsstore returns it; `ClassifyCreateError` classes it invalid; dataentry answers 422 `conflict_block` on create, PATCH, relation apply and in `writeInternalProblem`)
- [x] Regression test planned (data-entry test over a real file store for create, PATCH and relation content; unit test for the 500 backstop)
- [x] Related areas checked for similar issues (MCP and CLI print the error text, which is the sentinel's fixed message; batch needs an atomic store, so it never reaches the file store)
