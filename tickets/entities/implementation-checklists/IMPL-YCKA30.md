---
id: IMPL-YCKA30
type: implementation-checklist
title: 'Implementation: OpenAPI spec complete enough to drive rela-server from restish (attachments + auth)'
started: "2026-10-09"
completed: "2026-10-09"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code (handlers_attachment_raw_test.go, openapi/attachments_test.go)
- [x] Integration tests written (test full flow, not just units) (openapi_routes_test.go: every spec operation through the real router; wire-vs-spec entity fields)
- [x] Happy path implemented
- [x] Edge cases from planning handled (no name, malformed Content-Disposition, no Content-Length over cap, exactly at cap, path in name, query beats header)
- [x] Error handling in place (errors surfaced, not swallowed)

## Test Quality

- [x] Using fixture builders or factories for test data (newTestAppV1, seedEntity, writeACL)
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

Local stack: Pratique v26.10.0 (OAuth, DCR client) in front of rela-server
(`-jwt-header Authorization`, project with a `task` type holding an `evidence`
file property, PNG/JPEG only).

1. AC1, amended: discovery from the base URL is not possible, because the base
URL must be the origin and restish probes the root for a spec. The guide
configures `spec_files` explicitly. `restish api sync local` reports "Synced
spec" (before the change: 403 `origin_missing`), and `restish local --help`
lists `put-task-attachment`, `get-task-attachment`, `delete-task-attachment`
plus the CRUD commands.
2. AC2: the OAuth profile signs in through Pratique; `restish local list-tasks`
and the calls below run with the cached token. Unit test: TestSecurityScheme.
3. AC3: `restish local put-task-attachment TASK-3K9H evidence --filename
shot.png < shot.png` with a 401,232-byte PNG answers 200 with the entity
(`_attachments.evidence` lists it). `get-task-attachment` returns it and `cmp`
reports the files identical.
4. AC4: TestAttachmentUpload_RawBodyRefusals (400/413/422, nothing stamped),
TestAttachmentUpload_RawBodyGatedAndAudited (bob 404, carol 403 + audit, MIME
reject 422 + audit).
5. AC5: the live spec passes libopenapi-validator ValidateDocument (valid: true,
38 paths, 3.1.0), run from a scratch module, not added to go.mod. In-repo:
TestSpecIsStructurallySound (refs resolve, path params declared, unique
operationIds) and TestOpenAPI_EveryOperationReachesAHandler.

The e2e run also exposed spec drift that tests now pin: `EntityActions`
described objects where the wire sends booleans, and six per-entity wire fields
were missing (TestOpenAPI_EntitySchemaCoversWireFields).

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds) (problemContent() replaces the repeated
error content map)
- [x] No security issues introduced (raw names normalized by the shared attachment service; CSRF exemption keeps the no-Origin/no-cookie/no-Sec-Fetch condition; security review run in the review phase)
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
