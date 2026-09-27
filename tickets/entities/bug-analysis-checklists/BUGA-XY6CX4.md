---
id: BUGA-XY6CX4
type: bug-analysis-checklist
title: 'Analysis: pgstore range filters use database collation; empty ~= fails 42P18'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Reproduction

- [x] Bug reproduced locally (19 of 400 differential queries diverged on pgstore)
- [x] Minimal reproduction steps documented (a `~=` filter with an empty value; a `<` filter over values that differ only in case or punctuation)
- [x] Environment/conditions noted (PostgreSQL with an `en_US.UTF-8` database collation; `C` collation hides the second defect)

## Root Cause

- [x] Immediate cause identified (why1)
- [x] Contributing factors found (why2-3)
- [x] Systemic cause explored (why4-5)

## Fix Planning

- [x] Fix approach determined (return `TRUE` before binding; `COLLATE "C"` on range comparisons)
- [x] Regression test planned (the differential harness on pgstore)
- [x] Related areas checked for similar issues (sqlitestore compares with BINARY collation and binds after the early return; its differential passes)
