---
id: RR-FPZJX
type: review-response
title: 'Enforcement point lacks (from,to): ACL+validation run before oldEntity is loaded'
finding: The design's enforcement steps 2-4 assume the write chokepoint has both prior and new status. In Manager.UpdateEntity (entitymanager/manager.go:485) ACL authorizeAndAudit runs first (489) with no old state, then ValidateEntity(id,type,properties) runs on new state only (499), and oldEntity is loaded only after both, at 505 - passed only to automation/cascade/audit. So neither the ACL guard (403) nor validation (422) can see `from`. Also internal/validator is the wrong validator (offline batch read service, never gates a write); the real 422 is metamodel.ValidateEntity which also lacks old state. Not implementable without re-ordering the write path or hosting the check where OldEntity is available.
severity: critical
status: open
---

## Finding

The design's enforcement plan (steps 2–4) assumes the write chokepoint has both
the prior and new status in hand. It does not. In `Manager.UpdateEntity`
(`internal/entitymanager/manager.go:485`) the ordering is:

1. ACL `authorizeAndAudit` runs **first** (manager.go:489) — `WriteRequest{Op:
OpUpdate, Subject: {Type, ID}}`, no old state, no transition awareness.
2. `ValidateEntity(e.ID, e.Type, e.Properties)` runs next (manager.go:499) —
**new state only**; signature is `(id, type, properties)`
(`metamodel/validation.go:143`).
3. `oldEntity` is loaded **after** both, at manager.go:505, and is passed only to
the automation engine (`OldEntity`, line 518), autocascade (`OldTrigger`, line
561), audit summary, and unique-check.

So the two places the design wants to gate (ACL for the guard/403, validation
for legality/422) both execute before `from` is even known. The only write-path
components that receive prior state today are the **automation engine** and
**autocascade**.

Additionally, `internal/validator` (the batch `GenericValidator`) is the WRONG
validator — it is an offline store-scanning read service
(`validator/validator.go:1-9,176-193`), never runs inside the write path, and
emits reporting `Violation`s, not a write-blocking 422. The real 422 comes from
`metamodel.ValidateEntity` on the entitymanager path — which also lacks old
state.

## Impact

Legality (`from,to` must be a declared edge) and the guard (which permission a
given edge requires) both need `from`. As specced, neither enforcement point can
compute it. The design is not implementable without a structural change.

## Options to resolve (pick in design, before impl)

- **(A) Re-order the write path:** load `oldEntity` up front and thread it into
both the ACL request and a new transition-aware validation step. Cleanest for
semantics; touches the manager's hot path and the
`WriteRequest`/`ValidateEntity` contracts (old value + which edge). Note
`WriteRequest` has no field for a transition today (TKT-XZEY's "parameterised
verbs" is exactly this gap).
- **(B) Host the machine in the automation-engine tier**, which already gets
`OldEntity`/`Entity` and already computes `from`/`becomes`
(`automation/engine.go:210-214`). But automations are *reactions*, and the ACL
is not consulted there — the guard/403 would need the automation tier to call
the ACL, which it currently doesn't. Risks conflating reaction with gating (the
exact thing the ticket says to avoid).
- **(C) A dedicated transition-check step** in the manager between old-load and
store-write, calling ACL (subject-aware) + predicate. Most explicit; new code
but localized.

Recommend (A) or (C). Resolve the ordering + WriteRequest-shape question before
implementation; this is the load-bearing decision.
