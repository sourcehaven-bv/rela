---
id: BUGA-VJVOM0G
type: bug-analysis-checklist
title: 'Analysis: Fixed helper commands inherit REMOTE_CREDENTIAL_ variables'
started: "2026-11-14"
completed: "2026-11-14"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Reproduction

- [x] Bug reproduced locally (grep of exec.Command call sites; the new archguard test fails on the four sites before the fix)
- [x] Minimal reproduction steps documented (bug body lists the four call sites)
- [x] Environment/conditions noted (any host with a REMOTE_CREDENTIAL_ variable set)

## Root Cause

- [x] Immediate cause identified (why1)
- [x] Contributing factors found (why2-3)
- [x] Systemic cause explored (why4-5)

## Fix Planning

- [x] Fix approach determined (set Env from cmdexec.Environ at each site)
- [x] Regression test planned (archguard TestExecCommandsSetEnv)
- [x] Related areas checked for similar issues (all exec.Command sites in internal/ and cmd/; Lua has no os/io libs)
