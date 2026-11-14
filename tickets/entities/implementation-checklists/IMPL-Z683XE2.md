---
id: IMPL-Z683XE2
type: implementation-checklist
title: 'Implementation: OpenAPI spec lacks If-Match and 412 on the DELETE operations'
started: "2026-11-14"
completed: "2026-11-14"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code (TestGenerator_ConditionalWritesDeclareIfMatch)
- [x] Integration tests written (TestGoldenParity covers the served authenticated spec; goldens regenerated)
- [x] Happy path implemented
- [x] ~~Edge cases from planning handled~~ (N/A: spec-only change, no runtime behavior)
- [x] ~~Error handling in place~~ (N/A: no new code paths that can fail)

## Test Quality

- [x] ~~Using fixture builders or factories for test data~~ (N/A: the generator tests build a small metamodel literal, as the neighboring tests do)
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] ~~Interpolated values constructed from objects, not hardcoded~~ (N/A: no interpolated values)
- [x] ~~Property comparisons use original object, not hardcoded strings~~ (N/A: assertions check fixed header and status names)

## Manual Verification

- [x] Feature manually tested end-to-end (golden diff inspected: If-Match and 412 appear on every entity and relation DELETE)
- [x] Each acceptance criterion verified with test scenario from planning
- [x] ~~Edge cases manually verified~~ (N/A: spec-only change)

**Verification Evidence:** `go test ./internal/openapi/` passes; `go test
./internal/dataentry -run 'TestGoldenParity|OpenAPI'` passes after regenerating
the goldens; the golden diff is additions only.

## Quality

- [x] Code follows project patterns (copies the PATCH parameter and 412 response)
- [x] Checked for DRY opportunities (two short literals matching PATCH; not worth a helper)
- [x] No security issues introduced (documentation of existing behavior only)
- [x] ~~No silent failures~~ (N/A: no error paths)
- [x] No debug code left behind
