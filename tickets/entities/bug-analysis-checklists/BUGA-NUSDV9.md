---
id: BUGA-NUSDV9
type: bug-analysis-checklist
title: 'Analysis: sqlitestore compares RFC3339Nano timestamps as strings, which misorders values with trailing zeros'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Reproduction

- [x] Bug reproduced locally
- [x] Minimal reproduction steps documented
- [x] Environment/conditions noted

## Root Cause

- [x] Immediate cause identified (why1)
- [x] Contributing factors found (why2-3)
- [x] Systemic cause explored (why4-5)

## Fix Planning

- [x] Fix approach determined
- [x] Regression test planned
- [x] Related areas checked for similar issues

**Reproduction.** `TestFormatTimeSortsInTimeOrder` with `TimeFormat =
time.RFC3339Nano` fails its "trailing zeros" and "whole second" cases:
`…05.1234Z` sorts after `…05.123449999Z`, and `…05Z` after `…05.000000001Z`.
Pure Go, any platform, no database needed.

**Fix.** Write every compared timestamp column with the fixed-width UTC layout
already used by sqlitecomments (`sqlitedb.TimeFormat` via `FormatTime`), read
with RFC3339Nano (accepts both), and rewrite stored values in a v8 rung.

**Related areas.** sqlitecomments already fixed-width. project_files and
state_kv are written by other packages and never compared in SQL. A
caller-supplied non-UTC `UpdatedAt` was a second instance of the flaw;
`FormatTime` converts to UTC.
