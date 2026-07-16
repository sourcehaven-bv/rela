---
id: IMPL-A2CDP
type: implementation-checklist
title: 'Implementation: Resolved transition affordance: performable transitions for (principal, entity, field)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written (statemachine: Performable shape/gates, nil cases, MachineProps; affordances: TransitionVerdicts guard-held/denied/precondition/no-machines/terminal)
- [x] Integration tests (affordances TransitionVerdicts drives a real compiled Set + Declarative + stub graph end-to-end)
- [x] Happy path (performable transitions resolved for the ctx principal)
- [x] Edge cases (guard denied, precondition unmet/met, terminal state, non-machine prop, nil set/entity, no-machines-wired)
- [x] Error handling (verdicts carry Reason gate; no errors swallowed)

## Test Quality

- [x] Fixture builders (snapshotMeta/transitionMeta, ent/ticket, fakeGuard/fakeLookup, stubLookup)
- [x] No hardcoded values where object in scope
- [x] Only test-relevant values specified
- [x] Interpolated from objects
- [x] Property comparisons use the verdict struct, not loose strings

## Manual Verification

- [x] ~~Running-app manual test~~ (N/A: this ticket delivers the resolver method + accessor, no UI surface yet — the SPA control is a separate consumer ticket. Exercised end-to-end by the affordances integration test through a real compiled Set + Declarative.)
- [x] Each acceptance criterion verified by test (evidence below)
- [x] Edge cases via table tests

**Verification Evidence:**

- AC1 (Performable shape, sorted) — `TestPerformable_ShapeAndGates`
- AC2 (Allowed = guard∧precondition; Reason distinguishes) — same, three gate cases
- AC3 (nil for non-machine/terminal/nil) — `TestPerformable_NilCases`
- **AC4 (read/write parity, the drift guard)** — `TestPerformable_MatchesEnforceUpdate`: for every verdict, `Performable.Allowed == (EnforceUpdate succeeds)` and the named gate matches the error kind, across allow/guard-deny/precondition-fail. Backed by the shared `evalEdge` helper both `applyEdge` (write) and `Performable` (read) call.
- AC5 (affordances TransitionVerdicts via resolver ACL+graph) — `TestTransitionVerdicts_GuardHeld/GuardDenied/PreconditionGate`
- AC6 (subject-aware guard) — guard resolves via `acl.Request.HoldsPermissionForEntity` (the subject-aware path, already unit-proven at the acl layer by `TestRequest_HoldsPermissionForEntity_SubjectScoped`); affordances tests confirm the wiring honors held/not-held permissions.
- AC7 (bounded) — `Performable` takes one entity, iterates only that field's out-edges; no store/list scan; documented in godoc + CLAUDE.md.

CI: `go test ./...`, `-race` (statemachine/affordances), `golangci-lint ./...`
(0), `just arch-lint`, `just plimsoll`, `just coverage-check` (statemachine
85.3%, affordances 85.5%) — all green.

## Quality

- [x] Follows project patterns (consumer-side `Guard`/`GraphLookup`; `TransitionVerdicts` sibling of `FieldVerdicts`; guard adapter mirrors the write-path `transitionGuard`)
- [x] DRY (extracted `evalEdge` shared by write `applyEdge` and read `Performable` — the drift guard; `MachineProps`/`Performable` reuse `propType`/`edgeFor`)
- [x] No security issues (read-only; guard subject-aware via HoldsPermissionForEntity; nil request → no permission held, fails closed; bounded predicate eval)
- [x] No silent failures (verdicts carry the blocking gate; Compile errors surface at wiring)
- [x] No debug code

**Note (in-scope deviation):** the production resolver compiles its own `Set`
from the metamodel (`affordances_stub.go`) rather than threading the
entitymanager's single `Set` through `appbuild.Services`. Compile is a
deterministic pure transform so the two are identical; the read/write drift
guard is the shared `evalEdge`, not instance identity. Threading the shared Set
is the follow-up when the SPA-control ticket wires the whole surface (noted in
the code comment).
