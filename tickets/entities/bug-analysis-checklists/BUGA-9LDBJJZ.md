---
id: BUGA-9LDBJJZ
type: bug-analysis-checklist
title: 'Analysis: Entity page refresh after actions, step scroll, sidebar flash, run actor, unknown routes and 422 log'
started: "2026-11-21"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Reproduction

- [x] Bug reproduced locally (each defect reproduced in a vitest test that failed before the fix; the report came from a browser test of the saas-dev environment)
- [x] Minimal reproduction steps documented (in the bug description, one line per defect)
- [x] Environment/conditions noted (pool-rela behind the pratique edge and harbourmaster through the hub, with real latency and a signup bot)

## Root Cause

- [x] Immediate cause identified (why1)
- [x] Contributing factors found (why2-3)
- [x] Systemic cause explored (why4-5)

## Fix Planning

- [x] Fix approach determined (a revision prop the page bumps after an action; keepInView for `?run=`; a navLoaded gate in the sidebar; actorLabel shared with Changes; catch-all and /login routes; log only unexpected create failures)
- [x] Regression test planned (one vitest test per defect, plus the flow task hint e2e test)
- [x] Related areas checked for similar issues (the step form's catch logs nothing; FlowRunsPanel already reloads on live events and now on the revision too; the browser's own "Failed to load resource" line cannot be suppressed by the app)
