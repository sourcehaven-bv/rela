---
id: BUGA-YAVRNVC
type: bug-analysis-checklist
title: 'Analysis: Read-only record actions, ways out of a closed workspace, banner placement, people list and dark mail body'
started: "2026-11-22"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Reproduction

- [x] Bug reproduced locally (code reading against the test report: actionAllowed hid only create verbs, WorkspaceAccessScreen had no links, ReadOnlyBanner rendered in the main pane, maybeProvision ran only in write handlers, the dark block had no body rule)
- [x] Minimal reproduction steps documented (in the bug description)
- [x] Environment/conditions noted (multi-tenant server behind pratique's edge; read-only, suspended and archived tenants; provision policy)

## Root Cause

- [x] Immediate cause identified (why1)
- [x] Contributing factors found (why2-3)
- [x] Systemic cause explored (why4-5)

## Fix Planning

- [x] Fix approach determined (all write verbs hidden when read-only; shell meta for logo, links and contact; a banner slot on RlAppShell; POST _me provisions; body in the dark block)
- [x] Regression test planned (vitest and Go tests listed in prevention)
- [x] Related areas checked for similar issues (list row delete and inline edit go through actionAllowed; copy offers hidden with conversions; commands left to the server)
