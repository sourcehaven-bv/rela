---
id: BUGA-Q3M8TZ1
type: bug-analysis-checklist
title: 'Analysis: Session screen and confirms behind dialogs, 401 noise, tenant 404, action confirm colour'
started: "2026-11-22"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Reproduction

- [x] Bug reproduced locally (the built bundle declared --rl-z-modal:100 after the 1000 rebase; the old stacking test passed on it)
- [x] Minimal reproduction steps documented (open New Note, type, press Escape: the discard confirm is behind the dialog; end the session on a list page)
- [x] Environment/conditions noted (any production build; a single-tenant server for the 404)

## Root Cause

- [x] Immediate cause identified (why1)
- [x] Contributing factors found (why2-3)
- [x] Systemic cause explored (why4-5)

## Fix Planning

- [x] Fix approach determined (move the rebase to stacking.css after tokens.css; suppress error toasts and error blocks while accessRefusal is set; a single-tenant status route; pass danger from the action style)
- [x] Regression test planned (rlStacking.test.ts resolves the cascade; ui, EntityList, EntityDetail, toolbar and Go route tests)
- [x] Related areas checked for similar issues (every action confirm call site and every view with a load error block)
