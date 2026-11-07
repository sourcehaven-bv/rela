---
id: BUGA-71GBPLG
type: bug-analysis-checklist
title: 'Analysis: Two-tab autosave property test misses its out-of-attempts floor at random'
started: "2026-11-07"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Reproduction

- [x] Bug reproduced locally (15 local runs of 300: `exhausted` 34 to 50 against a floor of 30; the 200-seed loop on the old code finds seeds under it)
- [x] Minimal reproduction steps documented (run the floor test over many seeds and read the `exhausted` total; a debug log showed every armed exhaust got its three 412s, often spread over several saves)
- [x] Environment/conditions noted (any machine; the draw is random because `fc.assert` ran without a seed)

## Root Cause

- [x] Immediate cause identified (why1)
- [x] Contributing factors found (why2-3)
- [x] Systemic cause explored (why4-5)

## Fix Planning

- [x] Fix approach determined (bind the exhaust fault to one save; seed the floor run and name the seed in each floor message)
- [x] Regression test planned (the property test itself, checked over 200 seeds on old and new code)
- [x] Related areas checked for similar issues (useEntitySession.prop and EntityDetail.switching.prop also assert floors over unseeded samples; not changed here, they have not flaked)
