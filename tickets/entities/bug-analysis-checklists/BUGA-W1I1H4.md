---
id: BUGA-W1I1H4
type: bug-analysis-checklist
title: 'Analysis: Relation-conferred type@face grant denies explicitly addressed faces at the row gate'
started: "2026-10-07"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Reproduction

- [x] Bug reproduced locally
- [x] Minimal reproduction steps documented
- [x] Environment/conditions noted

## Root Cause

- [x] Immediate cause identified (why1)
- [x] Contributing factors found (why2-3)
- [x] Systemic cause explored (why4-5)

## Fix Planning

- [x] Fix approach determined
- [x] Regression test planned
- [x] Related areas checked for similar issues

## Notes

Reproduced on cf0fe87e^ (the commit before #1753) with TestComments_FaceLimitedConferredGrant:
GET `TKT-001@draft` returns 404 on the entity and comment routes under a
relation-conferred `ticket@draft` grant. On develop both return 200; the bare id
returns 404 on both, which is correct. Fix: none needed in code; add the
regression test. Related areas: PermitsReadFace shares
ReadableFacesMany with PermitsRead, so the two cannot disagree.
