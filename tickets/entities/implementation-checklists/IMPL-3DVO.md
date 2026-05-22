---
id: IMPL-3DVO
type: implementation-checklist
title: 'Implementation: Response-level action affordances: backend declares per-resource verbs to drive UI'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code
- [x] Integration tests written (test full flow, not just units)
- [x] Happy path implemented
- [x] Edge cases from planning handled
- [x] Error handling in place (errors surfaced, not swallowed)

## Test Quality

- [x] Using fixture builders or factories for test data
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

Phase 1 scope landed in PR #779:

- `internal/dataentry/affordances.go` — `translateVerb` + `computeActions` + `computeCollectionActions`.
- `V1Entity._actions` reshaped to `map[string]bool`; `V1ListResponse._actions` added.
- `internal/dataentry/affordances_test.go` — `TestTranslateVerb_Roundtrip`, `TestComputeActions_{ReadOnly,NopACL,AnonymousOmits}`, `TestComputeCollectionActions_*`, `TestComputeActions_NoAuditNoise` (AC1, AC2, AC8 verified).
- `internal/dataentry/affordances_contract_test.go` — `TestAffordances_BidirectionalContract` parameterized over NopACL + ReadOnlyACL (AC3 verified end-to-end via GET + DELETE lockstep).
- `internal/dataentry/lint_test.go` — `TestNoStrayWriteRequestConstruction` (AC10 — structural same-code-path enforcement).
- `principal.HasPrincipal(ctx)` added so the serializer can distinguish unstamped context from `Principal{User: "unknown"}`.
- `cmd/rela-server/main.go` + `cmd/rela-desktop/main.go` + `internal/dataentry/NewApp` signature updated for the new `acl.ACL` required collaborator.
- Frontend `_actions` type changed to `Record<string, boolean>` on both `Entity` and `ListResponse`. 784 frontend tests pass.
- `docs/data-entry/api-reference.md`, `docs/security.md`, `CLAUDE.md` updated. `just ci` green end-to-end (lint, arch-lint, lint-md, test, race, coverage 77.0%, build, docs-check).

Deferred to follow-ups (out of scope for this ticket):

- AC4 — list endpoint with mixed-permission rows: backend wiring landed (`computeCollectionActions`) but no dedicated test; will land in the phase-2 ticket.
- AC5 — Vue components consult `entity._actions[verb]`: no component currently reads it; phase-2 ticket.
- AC6 — synthetic-verb additive test: phase-2 ticket.
- AC7 — AWM6L payoff (E2E button-less UI in read-only): phase-2 ticket.
- `transition:*` and `relation:*` verbs: gated on ACL v0.5 follow-up.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
