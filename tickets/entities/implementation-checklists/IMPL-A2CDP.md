---
id: IMPL-A2CDP
type: implementation-checklist
title: 'Implementation: Resolved transition affordance: performable transitions for (principal, entity, field)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written (statemachine: Performable shape/gates, nil cases, MachineProps, self-loop omission; affordances: TransitionVerdicts guard-held/denied/precondition/no-machines/terminal)
- [x] Integration tests (affordances TransitionVerdicts drives a real compiled Set + Declarative + stub graph end-to-end)
- [x] Happy path (performable transitions resolved for the ctx principal)
- [x] Edge cases (guard denied, precondition unmet/met, terminal, non-machine prop, nil set/entity, no-machines, self-loop, value-dependent when:)
- [x] Error handling (verdicts carry Reason gate; no errors swallowed)

## Test Quality

- [x] Fixture builders (snapshotMeta/driftMeta/transitionMeta, ent/ticket, fakeGuard/fakeLookup, stubLookup)
- [x] No hardcoded values where object in scope
- [x] Only test-relevant values specified
- [x] Interpolated from objects
- [x] Property comparisons use the verdict struct

## Manual Verification

- [x] ~~Running-app manual test~~ (N/A: resolver method + accessor only, no UI/wire surface — see dormant-state note. Exercised end-to-end by the affordances integration test.)
- [x] Each acceptance criterion verified by test (evidence below)
- [x] Edge cases via table tests

**Verification Evidence:**

- AC1 shape/sorted — `TestPerformable_ShapeAndGates`
- AC2 allow=guard∧precond, Reason distinguishes — same
- AC3 nil non-machine/terminal/nil — `TestPerformable_NilCases`
- **AC4 read/write parity (drift guard) — `TestPerformable_MatchesEnforceUpdate`, now over snapshotMeta AND driftMeta (value-dependent `when: entity.value==to` + self-loop). Both code-review criticals (RR-QOZPX entity.value drift, RR-1WTST self-loop) would be red pre-fix.**
- AC5 affordances TransitionVerdicts — `TestTransitionVerdicts_*`
- AC6 subject-aware guard via HoldsPermissionForEntity (acl-layer proof `TestRequest_HoldsPermissionForEntity_SubjectScoped`)
- AC7 bounded — `Performable` one entity, out-edges only; documented

CI (post-review-fixes): `go test ./...`, `-race`, `golangci-lint ./...` (0),
`just arch-lint`, `just plimsoll`, `just coverage-check` — all green.

## Quality

- [x] Follows project patterns (consumer-side interfaces; TransitionVerdicts sibling of FieldVerdicts; guard adapter mirrors write-path)
- [x] DRY (shared evalEdge = the drift guard; MachineProps/Performable reuse internals)
- [x] No security issues (read-only; subject-aware guard; fail-closed on nil request; bounded predicate)
- [x] No silent failures
- [x] No debug code

## Code Review

- [x] `/code-review` (cranky) run; 6 findings, all addressed:
  - RR-QOZPX (critical) — read/write drift on entity.value preconditions → evaluate against post-transition entity. Fixed + parity test hardened.
  - RR-1WTST (critical) — self-loop performable-on-read → skip self-loops. Fixed + tested.
  - RR-RGG00 (significant) — weak parity fixture → driftMeta. Fixed.
  - RR-QOA1Z (significant) — guard-doc misclaim → corrected (fail-closed always, no inert tier, safe under active-policy-only).
  - RR-6CQYC (significant) — dormant query → disclosed (see below).
  - RR-2JBK4 (minor) — WithMachines/double-compile comments → tightened to state real assumptions.

**Dormant-state disclosure (RR-6CQYC):** this ticket delivers the read query
(`statemachine.Set.Performable`/`MachineProps` +
`PolicyResolver.TransitionVerdicts`) — the DATA. It has **no production caller
yet**: no dataentry handler emits it on the wire (`_transitions`), and the SPA
control does not exist. That is deliberate and matches the ticket's Scope (wire
surface + SPA control are the linked consumer ticket). The two criticals were
fixed regardless, so wiring it later is safe.
