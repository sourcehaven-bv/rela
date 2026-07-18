---
id: PLAN-J2VK7
type: planning-checklist
title: 'Planning: Resolved transition affordance: performable transitions for (principal, entity, field)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clear: expose "for principal X on entity Y, which transitions of field Z are performable right now" — the RESOLVED, authorized answer (guard held + precondition met), not the static declared edge set. This is the input a Linear/Jira status control needs.
- [x] Scope defined (see below)
- [x] Acceptance criteria documented (7 ACs in ticket)

**Scope IN:** (1) a resolving accessor `Set.Performable(ctx, e, prop, guard,
lookup) []TransitionVerdict` on `statemachine` that evaluates BOTH guard
(subject-aware) and `when:` (against the graph) for the single (principal,
entity), with per-edge allow + reason; (2)
`affordances.PolicyResolver.TransitionVerdicts(ctx, e)` beside
`FieldVerdicts`/`RelationVerdicts`, wiring the resolver's ACL + graph into (1).

**Scope OUT:** SPA status control (separate frontend ticket); `transition:*`
wire verb / `_actions` key (decide after the verdict shape is proven — this
produces the data); CLI + mermaid consumers; any write-path change.

**Corrected requirement (this replaces the original pure-accessor plan):** the
ask is the *resolved* set, not the static edge list. That folds guard + `when:`
evaluation in, which lives where the ACL + graph are (affordances), not in a
principal-blind accessor.

**Predicate-on-reads is fine here** — the rule targets unbounded/hot list paths;
a single field on a single entity is bounded O(edges). Clarified this boundary
in root + entitymanager CLAUDE.md this session so it doesn't mislead next time.

## Research

- [x] `affordances.PolicyResolver` already resolves predicates per single entity (`FieldVerdicts(ctx, e)`, `RelationVerdicts(ctx, e)` — resolver.go:308,385), already holds an `acl.Declarative` + `RelationLookup` (graph snapshot). `TransitionVerdicts(ctx, e)` slots in beside them — no new infrastructure.
- [x] `statemachine` internals reusable: `evalWhen` (predicate.go:59), `edgeFor` (statemachine.go), the `Guard`/`GraphLookup` consumer-side interfaces, and `acl.Request.HoldsPermissionForEntity` (subject-aware). The compiled `*predicate.Program` must stay encapsulated in `statemachine` (don't leak predicate to affordances) → the resolving method lives on `Set`, takes the collaborators, returns a plain DTO.
- [x] Write-path guard adapter (`appbuild.transitionGuard`) is the template for the affordance's guard adapter (over the resolver's `acl.Declarative`, current principal on ctx).

## Approach

- [x] **statemachine** — add:
  - `type TransitionVerdict struct { To, Guard string; Allowed bool; Reason string }` (public DTO; no `*predicate.Program`).
  - `func (s *Set) Performable(ctx, e *entity.Entity, prop string, guard Guard, lookup GraphLookup) []TransitionVerdict` — resolve machine via `propType`; `from = e.GetString(prop)`; for each out-edge from `from`: evaluate guard (via `guard.HoldsPermission`) then `when:` (via `evalWhen`), build verdict with reason; sort by To; nil for non-machine/terminal/nil-set.
  - **Extract a shared eval helper** so `applyEdge` (write) and `Performable` (read) call the SAME guard+when logic — AC4 pins they can't drift.
- [x] **affordances** — add `PolicyResolver.TransitionVerdicts(ctx, e) map[string][]statemachine.TransitionVerdict`: for each machine-typed field on e's type, call `Set.Performable` with a guard adapter over `r.declarative` and the resolver's `r.lookup`. (Resolver needs a handle to the compiled `Set` — inject it like the ACL/lookup are.)
- [x] Alternatives rejected: (a) principal-blind static accessor (the original plan) — doesn't answer the actual question; (b) leak compiled programs to affordances so it evaluates — couples affordances to `predicate` and duplicates the guard+when logic (drift risk); (c) evaluate in a new package — affordances already has exactly the collaborators, a new home would re-provision ACL+graph.
- [x] Files: `internal/statemachine/` (new `resolve.go` or into statemachine.go: DTO + `Performable` + shared helper; refactor `applyEdge` to use it), `internal/affordances/resolver.go` (+ Set injection, `TransitionVerdicts`), tests in both. Data-entry serializer wiring: minimal or deferred to the SPA-control ticket (this ticket delivers the resolver method + accessor; wire-surface is out-of-scope per ticket).

## Test Plan

- [x] AC1/2 (`Performable` shape + allow logic) — table: from `approved`, guard held + when-met → `{established, Allowed:true}`; guard not held → `{Allowed:false, Reason:guard}`; guard held but when-false → `{Allowed:false, Reason:precondition}`. Sorted by To.
- [x] AC3 — non-machine prop → nil; terminal state (`obsolete`) → nil; nil/empty Set → nil.
- [x] AC4 (read/write parity) — the key test: for the same (entity, edge, guard, graph), assert `Performable`'s `Allowed` == (`EnforceUpdate` succeeds). Drive both from the shared helper; a divergence fails. Cover allow, guard-deny, precondition-fail.
- [x] AC5 — `affordances` test: `TransitionVerdicts(ctx, e)` returns the right per-field verdicts using a stubbed ACL + graph (reuse affordances test harness).
- [x] AC6 (subject-scoped) — principal holding guard only via ownership relation to e → `Allowed:true`; without it → `Allowed:false, Reason:guard`. (Mirror `TestRequest_HoldsPermissionForEntity_SubjectScoped`.)
- [x] AC7 (bounded) — structural: `Performable` takes one entity, iterates its machine fields' out-edges only; no `ListEntities`/store scan reachable. Documented.
- [x] Integration: affordances-level test with a real compiled `Set` + `Declarative` proves the end-to-end resolve; no full-server E2E needed for this layer.

## Risk Assessment

- [x] Technical risk: moderate-low. The real risk is **read/write guard+when drift** — mitigated by the shared-helper refactor + the AC4 parity test (the single most important test here). Second risk: injecting the `Set` into `PolicyResolver` widens its constructor — small, follows the existing ACL/lookup injection.
- [x] Security risk: low, and this is the *positive-security* direction (it surfaces authorization the user has, computed by the ACL, never client-side). Read-only. Predicate-on-read is bounded (AC7). Guard is subject-aware (AC6) — no globals-only shortcut.
- [x] Effort: m (two packages, a shared-helper refactor of the write path, the parity test; bigger than the original s-estimate for the static accessor).

## Documentation Planning

- [x] Godoc on `TransitionVerdict`, `Set.Performable`, `PolicyResolver.TransitionVerdicts`.
- [x] The CLAUDE.md predicate-on-reads boundary clarified this session (root + entitymanager) — the "why this is allowed here" reference.
- [x] ~~docs/metamodel.md / API reference~~ (N/A: internal resolver method; user-facing surface arrives with the SPA-control consumer ticket).

## Design Review

- [x] ~~Run `/design-review`~~ — see decision. This is an `m` ticket touching the ACL-adjacent read path, but the design is fully specified, reuses established patterns (FieldVerdicts sibling; write-path guard adapter), and the one real risk (drift) is pinned by an AC-level parity test. Proportionate to skip formal design-review and rely on `/code-review` at the review phase. **Reconsider if implementation reveals the Set-injection into PolicyResolver forces a wider refactor than expected** — escalate to design-review then.
- [x] No critical/significant design findings at plan time.
