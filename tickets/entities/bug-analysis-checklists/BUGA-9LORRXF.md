---
id: BUGA-9LORRXF
type: bug-analysis-checklist
title: 'Analysis: TestHiddenProperty_NotCompared flakes: leak probe matches a digit of a generated id'
started: "2026-11-15"
completed: "2026-11-15"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Reproduction

- [x] Bug reproduced locally (`go test ./internal/mapimport -run TestHiddenProperty_NotCompared -count=30` fails every batch, with and without `-race`)
- [x] Minimal reproduction steps documented (in the bug description)
- [x] Environment/conditions noted (fails when the generated control id contains a 9; independent of `-race`)

## Root Cause

- [x] Immediate cause identified (why1)
- [x] Contributing factors found (why2-3)
- [x] Systemic cause explored (why4-5)

## Fix Planning

- [x] Fix approach determined (fixed id `CTL-1`, six-digit hidden value `987654`)
- [x] Regression test planned (the test itself, 200 runs under `-race`)
- [x] Related areas checked for similar issues (the only other substring probe in the package, `TestHiddenProperty_RealPolicy`, already uses a fixed id and `987`)
