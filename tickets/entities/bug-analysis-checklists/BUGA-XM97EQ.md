---
id: BUGA-XM97EQ
type: bug-analysis-checklist
title: 'Analysis: Type-mismatched relation creates skipped the audit log'
started: "2026-10-09"
completed: "2026-10-09"
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

## Notes

Reproduced by `TestTypeMismatchRelationWrite_Audited`: on the old code a
type-mismatched create through the data-entry API returns 200 and stores the
edge, but the audit sink holds 0 `OpCreateRelation` records.

Fix: a `RelationOptions.TolerateTypeMismatch` flag lets the manager accept the
soft condition on its normal path. Related areas checked: the update fallback
could never run (the manager's update path does not validate the allowlist);
MCP, CLI and Lua call the manager and return its error, so none has the
fallback. PR #1803 adds `writeBoundedFallback` with the same shape and should
use the flag.
