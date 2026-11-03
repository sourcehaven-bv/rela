---
id: BUGA-4K30LO
type: bug-analysis-checklist
title: 'Analysis: Pile writes fail in a non-default world; Create pile button is off-centre'
started: "2026-10-13"
completed: "2026-10-13"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Reproduction

- [x] Bug reproduced locally (Go test through attachWorld: POST /api/v1/_piles?world=published → 422 world_read_only)
- [x] Minimal reproduction steps documented (open a faced entity in a non-default world, Add to new pile…, Create pile)
- [x] Environment/conditions noted (any non-default world; the button layout is independent of the world)

## Root Cause

- [x] Immediate cause identified (why1)
- [x] Contributing factors found (why2-3)
- [x] Systemic cause explored (why4-5)

## Fix Planning

- [x] Fix approach determined (drop the world from the four pile write calls; RlButton stacks spinner + pending label in the label grid cell instead of reserving an idle slot)
- [x] Regression test planned (Go test of a pile write through attachWorld with an ID@face from a hidden face; usePiles test asserting writes send no world; RlButton test that an idle pending-label button has no spinner slot)
- [x] Related areas checked for similar issues (other world-scoped writes in api/*.ts; every pending-label button, e.g. DynamicForm Save, shares the off-centre layout)
