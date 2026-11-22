---
id: BUGA-7MO5HOT
type: bug-analysis-checklist
title: 'Analysis: Settings and About gating, a read-only workspace without pile changes'
started: "2026-11-22"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Reproduction

- [x] Bug reproduced locally (a local server with an acl.yaml role without settings:edit; a read-only workspace flag; an empty About dialog)
- [x] Minimal reproduction steps documented (open Settings as that role; open an entity page and a pile in a read-only workspace; open About)
- [x] Environment/conditions noted (acl.yaml role, read-only tenant)

## Root Cause

- [x] Immediate cause identified (why1)
- [x] Contributing factors found (why2-3)
- [x] Systemic cause explored (why4-5)

## Fix Planning

- [x] Fix approach determined (can_edit_settings on /_me; settings and about builtins rows; usePiles writable; app.about)
- [x] Regression test planned (Go me, builtins and config tests; vitest; e2e settings-gating)
- [x] Related areas checked for similar issues (the pile page, the entity list bulk action and the entity page pile actions use the same flag)
