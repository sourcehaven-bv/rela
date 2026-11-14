---
id: BUGA-69IZNNI
type: bug-analysis-checklist
title: 'Analysis: OpenAPI spec lacks If-Match and 412 on the DELETE operations'
started: "2026-11-14"
completed: "2026-11-14"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Reproduction

- [x] Bug reproduced locally (the generated spec for `/api/v1/{plural}/{id}` DELETE listed 204/404/409 and no If-Match; the relation DELETE listed 204/404)
- [x] Minimal reproduction steps documented (read `internal/openapi/paths.go` or the authenticated OpenAPI golden)
- [x] Environment/conditions noted (any backend; the spec is schema-derived)

## Root Cause

- [x] Immediate cause identified (why1)
- [x] Contributing factors found (why2-3)
- [x] Systemic cause explored (why4-5)

## Fix Planning

- [x] Fix approach determined (add If-Match and 412 to both DELETE operations, as PATCH has them)
- [x] Regression test planned (TestGenerator_ConditionalWritesDeclareIfMatch)
- [x] Related areas checked for similar issues (entity and relation PATCH already declare both; no other handler added If-Match in #2220)
