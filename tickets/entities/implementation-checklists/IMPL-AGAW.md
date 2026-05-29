---
id: IMPL-AGAW
type: implementation-checklist
title: 'Implementation: v1 entity create bypasses field-affordance write gate'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code — `TestHandleV1CreateEntity_FieldAffordances` (hidden / unknown / read-only / enum-filtered / allowed) + `TestHandleV1CreateEntity_AffordanceDenial_EmitsAudit`.
- [x] Integration tests written (test full flow, not just units) — tests drive the real `handleV1CreateEntity` HTTP handler end-to-end (httptest), asserting status + wire `rule_id` + audit row, not the validator in isolation.
- [x] Happy path implemented — allowed fields create succeeds (201).
- [x] Edge cases handled — read-only field omitted from body never trips the gate; unknown field rejected with hidden-shape (F8 parity); nil `properties` is safe.
- [x] Error handling in place — denial routes through `denyAffordance` (403 + `denied-write` audit), identical to the PATCH path.

## Test Quality

- [x] Using fixture builders or factories for test data — `newVerdicts().ReadOnly(...).EnumDeny(...).Build()` and `createTicketRaw` helper.
- [x] No hardcoded values in assertions when object is in scope — rule_id strings are the wire contract (appropriate to assert literally, per CLAUDE.md "format validation" exception).
- [x] Only specifying values that matter for the test — each subtest sets only the denied dimension.
- [x] ~~Interpolated values constructed from objects~~ (N/A: no interpolated values in these assertions).
- [x] ~~Property comparisons use original object~~ (N/A: these tests assert HTTP status / rule_id, not preserved properties).

## Manual Verification

- [x] Feature manually tested end-to-end — exercised via the HTTP-level tests (httptest recorder against the real handler); `just ci` green.
- [x] Each acceptance criterion verified — create now 403s denied fields with the same rule_id as PATCH; allowed create succeeds; denial audited.
- [x] Edge cases manually verified — covered by subtests (unknown field, read-only omitted vs set, enum option).

**Verification Evidence:** `go test ./internal/dataentry/ -run
'TestHandleV1CreateEntity_FieldAffordances|TestHandleV1CreateEntity_AffordanceDenial_EmitsAudit'`
→ PASS (6 subtests + audit). Full `just ci` → exit 0 (lint, arch-lint, test,
coverage, e2e, docs all green).

## Quality

- [x] Code follows project patterns — mirrors the existing PATCH gate call site (`validateFieldWrite` + `denyAffordance`); no new imports (reused `entityPkg`).
- [x] Checked for DRY opportunities — reused the existing `validateFieldWrite` gate rather than duplicating denial logic; create vs update still call it independently (the deduplication into a single choke point is the systemic follow-up noted in `prevention`/why5, not in scope here).
- [x] No security issues introduced — change is fail-closed; tightens create to match PATCH.
- [x] No silent failures — denials return 403 and emit an audit record.
- [x] No debug code left behind.
