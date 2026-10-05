---
id: BUGA-TFGPTV
type: bug-analysis-checklist
title: 'Analysis: Relation reads and writes mishandle faced endpoints'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Reproduction

- [x] Bug reproduced locally
- [x] Minimal reproduction steps documented
- [x] Environment/conditions noted

Each listed defect was checked against the faces-intrinsic branch after the
TKT-2528AB PRs merged. `FilterRelations`, relation create/GET to a faced target,
the clone and relation-write oracle and MCP show were already fixed (tests
below). Two surfaces still served content edges with a face that does not own
them, reproduced with Go tests on the faced `policy` fixture (`contentEdgeApp`):
the command payloads (`relationsForEntity` and the view payload listed every
incident edge, with no peer gate) and the gantt edge filter. Conditions: a type
declaring `faces:` and a relation with `scope: content`.

## Root Cause

- [x] Immediate cause identified (why1)
- [x] Contributing factors found (why2-3)
- [x] Systemic cause explored (why4-5)

## Fix Planning

- [x] Fix approach determined
- [x] Regression test planned
- [x] Related areas checked for similar issues

Approach: command payloads keep an outgoing content edge only with the served
face (`ownedByFace`) and peer-gate every edge with one
`Resolver.EndpointsReadableErr` batch; a read fault fails the command instead of
sending an empty list. The view payload reads its edges in one query and keeps
content edges only for a face it serves. The gantt node records its face and
`ganttEdgesForType` drops edges its face does not own, before the fold. Related
areas: MCP `list_relations` and Lua `get_relations` query by bare id and return
`from_face` (no fix needed); the unlink default tail moves to Stage 2
(BUG-J3PBFN, TKT-KQXVF7); the list-context payload stays with TKT-2FDTJE.
