---
id: BUGA-J71OT58
type: bug-analysis-checklist
title: 'Analysis: Flow failure cause, notices and page follow-ups, and a flow runs panel gate'
started: "2026-11-22"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Reproduction

- [x] Bug reproduced locally (a local server on a copy of the saas-dev pool config; RUN-CNPM inspected on saas-dev)
- [x] Minimal reproduction steps documented (a job with a permanent error; open a run's ?run= page; a read-only workspace with an empty items_from group; a list at 375px with empty cells)
- [x] Environment/conditions noted (phone width, read-only workspace, portal role)

## Root Cause

- [x] Immediate cause identified (why1)
- [x] Contributing factors found (why2-3)
- [x] Systemic cause explored (why4-5)

## Fix Planning

- [x] Fix approach determined (ViaFailed log step; leadsHere; filter empty groups; refreshNow; fixed compressed cell width; menu bar title; widget label; entity_panels toggle enforced on the subject runs route)
- [x] Regression test planned (Go engine and handler tests, vitest, e2e mobile)
- [x] Related areas checked for similar issues (the exhausted-retries path also writes the failure step; the run list and run page keep their own access rules)
