---
id: BUGA-8SIEHX9
type: bug-analysis-checklist
title: 'Analysis: Rejected channel batches are not charged against the rate limit'
started: "2026-11-14"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Reproduction

- [x] Bug reproduced locally (TestPostBatch_Charges got 400 instead of 429 on the second invalid batch before the fix)
- [x] Minimal reproduction steps documented (post the same invalid batch twice on a type with rate 3/m; both return 400)
- [x] Environment/conditions noted (any backend; the limiter is process-local)

## Root Cause

- [x] Immediate cause identified (why1)
- [x] Contributing factors found (why2-3)
- [x] Systemic cause explored (why4-5)

## Fix Planning

- [x] Fix approach determined (charge right after the generation check, as Post does)
- [x] Regression test planned (TestPostBatch_Charges)
- [x] Related areas checked for similar issues (Post, signals, Edit, Remove, lifecycle, actions, typing and create all charge before validation)
