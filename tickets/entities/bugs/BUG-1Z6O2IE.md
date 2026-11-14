---
id: BUG-1Z6O2IE
type: bug
title: OpenAPI spec lacks If-Match and 412 on the DELETE operations
description: 'The generated OpenAPI spec omitted the If-Match header and the 412 response on entity and relation DELETE, although the handlers honor If-Match since #2220 (GH-2228).'
priority: low
why1: The OpenAPI spec listed no If-Match parameter and no 412 response on the entity and relation DELETE operations.
why2: '#2220 (TKT-CVT9W1K) added If-Match handling to both DELETE handlers but did not change internal/openapi/paths.go.'
why3: The spec is hand-assembled in Go next to, not from, the handlers, so a handler change does not show in the spec unless someone edits it.
why4: No test tied the spec's conditional-write declarations to the handlers that honor If-Match; the goldens only record whatever the generator emits.
why5: Spec completeness for request headers and status codes depends on review rather than a check.
prevention: TestGenerator_ConditionalWritesDeclareIfMatch requires If-Match and 412 on the entity and relation PATCH and DELETE operations, so removing either from a conditional write fails the build.
started: "2026-11-14"
completed: "2026-11-14"
status: done
---

## Symptom

GitHub issue #2228. Since #2220 (TKT-CVT9W1K) a DELETE on an entity or on a
single relation honors `If-Match` and answers 412 `precondition_failed` on a
stale value. The spec from `internal/openapi/paths.go` listed neither the header
nor the 412 for those two operations.

## Fix

Add the `If-Match` header parameter and the 412 response to both DELETE
operations, as PATCH has them. Regenerate the authenticated OpenAPI goldens.
`TestGenerator_ConditionalWritesDeclareIfMatch` pins If-Match and 412 on the
entity and relation PATCH and DELETE operations.
