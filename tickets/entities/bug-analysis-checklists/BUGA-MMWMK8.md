---
id: BUGA-MMWMK8
type: bug-analysis-checklist
title: 'Analysis: sqlite swept versions lose copy provenance (no origin columns on live rows)'
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

**Reproduction.** `storetest.RunSweepOriginTests` against sqlitestore without
the sweep change: both cases fail (the swept version has a zero Origin).

**Fix.** Add the five `origin_*` columns to sqlite `entities` (schemaSQL plus a
v9 rung), stamp `store.OriginFrom(ctx)` on create and update, and copy them in
the sweep, as pgstore does since migration 0013.

**Related areas.** Relations carry no origin on either backend (a copy writes
entity faces). The synchronous capture path already wrote origin on sqlite.
The pgstore-only sweep origin tests moved into the shared suite.
