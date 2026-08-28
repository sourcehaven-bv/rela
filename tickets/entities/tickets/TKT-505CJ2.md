---
id: TKT-505CJ2
type: ticket
title: 'Fix TransitionDef.Guard: an ACL permission declared in schema.yaml that is inert on non-served write paths'
kind: enhancement
priority: medium
effort: m
status: backlog
---

## Blocked on / follow-up to

**TKT-XZEY** — circle back to this once per-relation CUD permissions land. XZEY
establishes the principle this ticket applies; doing them in the other order
would mean deciding the principle twice.

## Problem

`metamodel.TransitionDef.Guard` (`internal/metamodel/types.go:200`) names an ACL
permission from inside `schema.yaml`. Its own godoc concedes the defect:

```go
// Guard names an ACL permission the acting principal must hold for this
// transition. Enforced only on served paths (a principal exists); inert
// on direct CLI writes. Empty means the transition is legal for anyone
// who may otherwise write the entity.
Guard string `yaml:"guard,omitempty"`
```

A security gate that **silently does not apply depending on entry point** is a
bug, not a documented limitation. An operator reading `guard: establish` in
`schema.yaml` has no cue that the guard evaporates on `rela` CLI writes, the
scheduler, or any other path where no principal is present. It reads as an
enforced constraint and is not one.

The root cause is structural: the metamodel is loaded on **every** path, while
the ACL is not. So a metamodel field making a policy claim can only ever be
best-effort.

## The principle (established in TKT-XZEY)

**Only a layer that already depends on both schema and ACL may reference both.**

| File | References schema? | References ACL? | Verdict |
|---|---|---|---|
| `data-entry.yaml` | yes | yes (4 `Permission` fields) | **fine** — composition layer, that is its job |
| `acl.yaml` | by name only (relation/entity type names) | yes | **fine** — names are not topology |
| `schema.yaml` | yes | `guard:` | **bug** — domain model must be enforceable with no ACL present |

`schema.yaml` is the domain model. It must stay readable and enforceable when no
ACL exists at all.

## Directions (not yet decided)

1. **Move the declaration to `acl.yaml`**, keyed by (type, field, transition) —
mirroring the `relations: <name>: <verb>: <permission>` shape XZEY introduces.
Schema keeps the transition *topology* (`from`/`to`/`when`); the ACL keeps who
may perform it. Cleanest fit with the principle.
2. **Make it fail-closed on unprincipalled paths.** Keeps the field where it
is but inverts the default: no principal → guarded transitions are denied rather
than allowed. Breaks CLI/scheduler workflows that currently work, so it needs a
migration story.
3. **Demote it to advisory and rename it.** If it can only ever be
best-effort, stop calling it a guard. Weakest option — it documents the problem
instead of fixing it.

(1) is the presumptive direction given XZEY, but the CLI/scheduler impact of
moving enforcement needs checking before committing.

## Open questions

- **Blast radius.** How many deployments use `guard:` today, and on which
paths do those transitions actually get written? If the answer is "served paths
only, always", (2) becomes much cheaper.
- **Interaction with `HoldsPermissionForEntity`.** The guard is checked via
the entity-scoped permission path (TKT-E4LW2, `internal/acl/resolver.go`),
unlike `holdsPermission` which is global-only. Whichever direction wins must
preserve that per-entity scoping — a transition guard genuinely needs "may
establish *this* entity", not a flat global capability.
- **`When:` stays put regardless.** The predicate precondition is a data
constraint, not a policy one, and belongs in the metamodel. Only `Guard:` is in
question.

## Out of scope

- The transition/state-machine model itself (`TransitionDef.From/To/When`,
`internal/statemachine`).
- The four `data-entry.yaml` `Permission` fields — those are correct as-is.
