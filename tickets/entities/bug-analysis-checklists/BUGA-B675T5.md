---
id: BUGA-B675T5
type: bug-analysis-checklist
title: 'Analysis: Create form cannot save a pre-linked peer whose type uses a dashed id_prefix'
started: "2026-10-06"
completed: "2026-10-06"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Reproduction

- [x] Bug reproduced locally (e2e create-prelink.spec.ts fails with the reported toast)
- [x] Minimal reproduction steps documented (type with id_prefix MOD-; view section with create: {} over an outgoing relation; target form without a field for it; create with a title only)
- [x] Environment/conditions noted (any backend; data-entry SPA; modal and page flow)

## Root Cause

- [x] Immediate cause identified (why1)
- [x] Contributing factors found (why2-3)
- [x] Systemic cause explored (why4-5)

## Fix Planning

- [x] Fix approach determined (shared entityTypeForId helper: id_prefixes or id_prefix, either spelling, longest match wins)
- [x] Regression test planned (unit tests for the helper; e2e section create with a dashed-prefix peer)
- [x] Related areas checked for similar issues (only DynamicForm derives a type from an id in the frontend; backend MatchesID and analyze handle both spellings)
