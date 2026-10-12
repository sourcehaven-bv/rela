---
id: BUGA-T6CGTL
type: bug-analysis-checklist
title: 'Analysis: Write responses name neighbours the caller cannot read'
started: "2026-10-12"
completed: "2026-10-12"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Reproduction

- [x] Bug reproduced locally (three failing tests on develop 0d5baf21)
- [x] Minimal reproduction steps documented (write_response_neighbors_test.go: alice reads ticket+feature, not secret; TKT-1 or the created ticket links SEC-1)
- [x] Environment/conditions noted (fs store; restore uses the test history double; any backend shares the handler code)

## Root Cause

- [x] Immediate cause identified (why1)
- [x] Contributing factors found (why2-3)
- [x] Systemic cause explored (why4-5)

## Fix Planning

- [x] Fix approach determined (read edges through servedFaceEdges and serialize with forWireScoped, as the GET does)
- [x] Regression test planned (one test per response)
- [x] Related areas checked for similar issues (PATCH already uses faceEdges; export.go and pagescope.go keep their own gates and are out of scope)
