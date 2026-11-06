---
id: BUGA-AXMPFBD
type: bug-analysis-checklist
title: 'Analysis: A body line starting with a conflict marker makes the file unreadable'
started: "2026-11-05"
completed: "2026-11-05"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Reproduction

- [x] Bug reproduced locally (storetest RoundTripsBodyDocumentingConflictMarkers failed on fsstore before the fix)
- [x] Minimal reproduction steps documented (create an entity whose body has a paragraph starting with the start marker, then read it back)
- [x] Environment/conditions noted (fsstore only; other backends store bodies in columns)

## Root Cause

- [x] Immediate cause identified (why1)
- [x] Contributing factors found (why2-3)
- [x] Systemic cause explored (why4-5)

## Fix Planning

- [x] Fix approach determined (one scanner requiring start, separator and end markers in order, outside fenced code; fsstore, conflict list and resolver share it)
- [x] Regression test planned (storetest round trip, markdown.ConflictBlocks table, fsstore parse cases, resolver keeps a fenced example)
- [x] Related areas checked for similar issues (conflict.FindMarkers, RemoveConflictMarkers, markdown.ParseDocument, KeyNode quoting)
