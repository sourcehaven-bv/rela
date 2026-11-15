---
id: BUGA-JI21Q84
type: bug-analysis-checklist
title: 'Analysis: TestHiddenProperty_NotCompared fails when a random entity id contains 9'
started: "2026-11-15"
completed: "2026-11-15"
status: done
---

## Reproduction

- [x] Bug reproduced locally (20 of 200 runs failed with `go test -count=200 -run TestHiddenProperty_NotCompared ./internal/mapimport/`)
- [x] Minimal reproduction steps documented (command above)
- [x] Environment/conditions noted (any platform; fails when the generated control id contains a 9)

## Root Cause

- [x] Immediate cause identified (why1)
- [x] Contributing factors found (why2-3)
- [x] Systemic cause explored (why4-5)

## Fix Planning

- [x] Fix approach determined (8-digit sentinel value no id can contain, shared assertHidden helper)
- [x] Regression test planned (the test itself, run 200 times)
- [x] Related areas checked for similar issues (TestHiddenProperty_RealPolicy had the same pattern with 987 and a fixed id; it now uses the sentinel too)
