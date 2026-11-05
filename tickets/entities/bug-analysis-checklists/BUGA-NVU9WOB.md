---
id: BUGA-NVU9WOB
type: bug-analysis-checklist
title: 'Analysis: Conflict list and detail endpoints skip the read gate'
started: "2026-11-04"
completed: "2026-11-04"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Reproduction

- [x] Bug reproduced locally (regression tests in internal/dataentry/conflicts_api_test.go fail on develop)
- [x] Minimal reproduction steps documented (principal with git:sync and read on ticket only; conflicted feature file)
- [x] Environment/conditions noted (fs backend with git sync; conflicted files only)

## Root Cause

- [x] Immediate cause identified (why1)
- [x] Contributing factors found (why2-3)
- [x] Systemic cause explored (why4-5)

## Fix Planning

- [x] Fix approach determined (gate the identities a conflicted file names through the row gate; resolve an entity through entitymanager AuthorizeUpdate)
- [x] Regression test planned
- [x] Related areas checked for similar issues (the resolve POST shared the gap; both fixed together)
