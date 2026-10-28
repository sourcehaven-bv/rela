---
id: BUGA-ZSOWDM
type: bug-analysis-checklist
title: 'Analysis: Data-entry answers 500 instead of 401 when a handler finds no bound identity'
started: "2026-10-28"
completed: "2026-10-28"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Reproduction

- [x] Bug reproduced locally (probe test over registerAPIV1Routes with a Declarative ACL wired into the write path and no acl.Request on ctx: PATCH 500 acl_query_failed, DELETE 500 delete_failed, POST 403 with reason "no request scope")
- [x] Minimal reproduction steps documented (bug description; pinned by internal/dataentry/noidentity_status_test.go)
- [x] Environment/conditions noted (only reachable when a handler runs without the attachACLRequest edge, for example a test or a future route that skips it; production /api routes bind identity first)

## Root Cause

- [x] Immediate cause identified (why1)
- [x] Contributing factors found (why2-3)
- [x] Systemic cause explored (why4-5)

## Fix Planning

- [x] Fix approach determined (one helper, writeNoIdentity, called from the shared mappers writeGateError, writeInternalProblem, writeInternalJSONError and writeForbiddenIfACLDenied; acl.Decision gains a Cause the ForbiddenError unwraps to)
- [x] Regression test planned (table-driven over GET entity, GET list, POST, PATCH, DELETE)
- [x] Related areas checked for similar issues (identity mismatch and foreign identity also reached 403 on POST; now 500 like PATCH and DELETE)
