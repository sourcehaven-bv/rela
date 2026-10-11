---
id: BUGA-0J105J
type: bug-analysis-checklist
title: 'Analysis: Search returns hits whose only match is a visible:-hidden field on the bleve backend'
started: "2026-10-11"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Reproduction

- [x] Bug reproduced locally
- [x] Minimal reproduction steps documented (TestSearchVisible_BleveHiddenOnlyMatchDropped)
- [x] Environment/conditions noted (default fs build, bleve backend, a role with a `visible:`-hidden property)

## Root Cause

- [x] Immediate cause identified (why1)
- [x] Contributing factors found (why2-3)
- [x] Systemic cause explored (why4-5)

## Fix Planning

- [x] Fix approach determined
- [x] Regression test planned
- [x] Related areas checked for similar issues (pgstore computes provenance separately with MatchTextFields; LinearSearch matches by substring, so its substring provenance is exact)
