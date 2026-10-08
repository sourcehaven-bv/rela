---
id: BUGA-VQWC3I
type: bug-analysis-checklist
title: 'Analysis: Rename and delete errors reveal hidden entities and their relations'
started: "2026-10-07"
completed: "2026-10-07"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Reproduction

- [x] Bug reproduced locally (failing tests in internal/entitymanager/hidden_write_errors_test.go before the fix)
- [x] Minimal reproduction steps documented (each test seeds one entity the caller cannot read and asserts the write outcome)
- [x] Environment/conditions noted (memstore; declarative ACL where the caller cannot read decisions)

## Root Cause

- [x] Immediate cause identified (why1)
- [x] Contributing factors found (why2-3)
- [x] Systemic cause explored (why4-5)

## Fix Planning

- [x] Fix approach determined (accept the one collision bit and document it; neutral cascade denial; readable-only RelationsUpdated in the manager; rename only for id_type manual)
- [x] Regression test planned (manager tests per leak; affordance test; MCP golden)
- [x] Related areas checked for similar issues (MCP delete already counts visible edges; Lua and CalDAV pass through the manager errors; data-entry 403 drops the wrapped text)
