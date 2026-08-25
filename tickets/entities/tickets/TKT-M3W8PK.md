---
id: TKT-M3W8PK
type: ticket
title: 'Gate automation relation writes as the triggering principal (+ autocascade fatal-error contract)'
kind: enhancement
status: backlog
priority: high
effort: l
---

## Blocked on

**TKT-7QM4RB** (carry `allow_acl_bypass` through the declarative
`create_relation:` action). Without it this change has **no operator opt-out**.

Split out of **TKT-XZEY** at design review (DR-2). See that ticket for the
per-relation permission model this complements.

## The gap

`cascadeHost.WriteRelation` (`entitymanager/cascadehost.go:140-155`) calls
`Store.CreateRelation` **directly — no ACL call at all**, audit only. Automations
create arbitrary relation types with zero relation-level authorization.

This inverts the codebase's own pattern: a **Lua** action must DECLARE
`allow_acl_bypass` to skip the gate, while the **declarative** `create_relation:`
action skips it unconditionally. `create_entity` is not a counter-example —
`cascadeHost.CreateEntity` routes through `createCore`, and `WriteEntity`
(`:91`) re-runs validation, unique checks and transition enforcement, with the
comment *"the create path must not be the weaker one"* (BUG-KIMZRK). Only
`WriteRelation` does neither.

## Decision

Gate it as the **triggering principal** — the principal whose write started the
cascade. No implicit elevation. Elevation requires an explicit
`allow_acl_bypass` on the action (TKT-7QM4RB).

**Verified feasible:** the principal IS on the ctx at `WriteRelation`. The chain
`Manager.CreateEntity/UpdateEntity` → `Cascade.Process` (`manager.go:561,824`) →
`Runner.Process` (`autocascade/runner.go:64`) → `applyRelationCreations` (`:96`)
→ `host.WriteRelation` (`:230`) is unbroken — no `context.Background()`, no
`WithoutCancel`. Proof it already works: `recordCascade` reads
`principal.From(ctx)` at `cascadehost.go:261` for correct audit attribution.

## The hard part — the fatal-error contract

A denial must **abort the cascade and surface the error**, never
skip-and-continue. Neither half works today:

**(a) The runner cannot abort.** `applyRelationCreations` swallows every
`WriteRelation` error into a string and `continue`s
(`autocascade/runner.go:231-234`), same at `:363-367`. `Runner.Process` returns
non-nil error ONLY for a nil trigger (`:68-70`).

**(b) The error is discarded on 3 of 4 transports.** `outcome.Errors` folds into
`result.AutomationErrors` (`manager.go:568,837`) and the write returns nil. The
COMPLETE list of non-test consumers is `cli/create.go:71` and `cli/update.go:105`.
**`internal/dataentry`, `internal/mcp` and `internal/lua` have zero references.**

So a naive implementation yields, on SPA/MCP/**Lua**: HTTP 200 / success, edge
silently absent, no error anywhere — a **bit-for-bit reproduction of the
motivating outage on the same Lua transport**, caused by the fix.

### Required

1. `applyRelationCreations` / `createTriggerRelation` return an error, and
   `Runner.Process` propagates it, **when the failure is an authorization
   denial specifically** (`errors.As` for `*acl.ForbiddenError`). Keep
   skip-and-continue for pre-existing non-security failures (target-not-found,
   metamodel-invalid) so this is not a blanket behaviour change.
   `internal/autocascade` may not import `metamodel` (`runner.go:278`) — check
   whether the same applies to `internal/acl`; if so define a narrow
   consumer-side `interface{ IsAuthorizationDenial() bool }` at the runner.
2. `Outcome` needs a `FatalErr error` field alongside `Errors []string`.
3. `Manager.CreateEntity`/`UpdateEntity` return it as a real `error` — the
   `cascadeErr` branch at `manager.go:569-571` is one propagation away.
4. Dataentry error mapping: a mid-cascade abort becomes a 403.
5. A denial must record a denied-write audit row. `recordCascade`
   (`cascadehost.go:248-265`) has no denial branch — add one to `cascadeHost`.
   Do **NOT** give it a `*Manager` back-reference; that re-creates the
   elevation-propagation hazard `gated()` prevents (`manager.go:98-103`).

## Honest limitation

Abort does **not** give atomicity. On fs/mem there is a write mutex, not a
transaction (`fsstore/tx.go:60-64`), so earlier cascade steps stay committed:
"entity created, some relations created, one denied, error returned". That is a
**loud** partial write replacing a **silent** one — a real improvement, not a fix
for non-atomicity. Add an AC asserting exactly which writes persist.

## Residual, deliberately not closed

The gate keys on relation TYPE, never TARGET. `targetID :=
e.interpolate(action.CreateRelation.To, event)` (`automation/engine.go:285-289`)
expands `{{new.<prop>}}`, so a low-privilege user setting a property on the
trigger entity **chooses the `To` endpoint**. `RelationSubject` has no `ToID`
(RR-F9M9, deliberate — do not re-litigate). Document the residual.

## Migration break

An automation that works today starts failing when the triggering user lacks the
grant. Correct semantics, but a real behaviour change needing a release note.
Escape hatch is `allow_acl_bypass` (TKT-7QM4RB).

## Acceptance criteria

1. Automation relation-create denied when the triggering principal lacks the
   grant, cascade **aborts**, error surfaces **at the transport** (HTTP 4xx /
   MCP error / Lua raise) — not just in `AutomationErrors`.
2. An action declaring `allow_acl_bypass` (write) still creates the relation.
3. Non-security cascade errors still skip-and-continue (no blanket change).
4. Partial-write state after a mid-cascade denial is asserted explicitly.
5. A denial produces a denied-write audit row.
