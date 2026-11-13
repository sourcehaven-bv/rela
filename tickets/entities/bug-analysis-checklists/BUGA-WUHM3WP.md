---
id: BUGA-WUHM3WP
type: bug-analysis-checklist
title: 'Analysis: Two-tab autosave mutant test rejects one of the two blind-write reports'
started: "2026-11-12"
completed: "2026-11-12"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Reproduction

- [x] Bug reproduced locally (15 instrumented runs of both mutant tests logged the violation kinds; some failing runs report a body blind write only as "wrote c:… which it does not own", the CI failure needs every failing run of one property call to do so)
- [x] Minimal reproduction steps documented (CI run 43078734201; the failure message lists the violations found, none matching `overwrote c:`)
- [x] Environment/conditions noted (any machine; `property()` in the mutant tests runs without a seed)

## Root Cause

- [x] Immediate cause identified (why1)
- [x] Contributing factors found (why2-3)
- [x] Systemic cause explored (why4-5)

## Fix Planning

- [x] Fix approach determined (assert that a blind write to the broken field kind was reported, with either `checkWrite` message; pinning a seed was rejected because it would test one sample instead of the property)
- [x] Regression test planned (the two mutant tests themselves; the file run 30 times)
- [x] Related areas checked for similar issues (the property-merge mutant test had the same `overwrote p:` assertion and gets the same fix)
