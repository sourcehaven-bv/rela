---
id: BUG-KIMZRK
type: bug
title: cascadeHost.WriteEntity persists automation-set properties directly to the store, bypassing ACL, validation, unique checks, transitions and audit
description: internal/entitymanager/cascadehost.go:79-84 calls h.deps.Store.UpdateEntity directly instead of Manager.UpdateEntity, so cascade re-writes of automation-set properties skip ACL authorization, metamodel validation, unique-constraint checks, state-machine transition enforcement, and the audit log. Contradicts the root CLAUDE.md rule that all writes go through the Manager. The sibling top-level create path (manager.go:456-476) explicitly re-checks uniques post-automation with the reasoning that it 'must not be the weaker one' — the cascade path has no equivalent. Found while investigating TKT-80EWGM.
priority: high
status: backlog
---

## Summary

`cascadeHost.WriteEntity` (`internal/entitymanager/cascadehost.go:79-84`) writes
straight to the raw store:

```go
func (h *cascadeHost) WriteEntity(ctx context.Context, e *entity.Entity) error {
	if e == nil { return nil }
	return h.deps.Store.UpdateEntity(ctx, e)
}
```

It does **not** go through `Manager.UpdateEntity`, so this write skips the
entire write pipeline:

- ACL authorization (`authorizeAndAudit`)
- metamodel validation (`Meta.ValidateEntity`) and DEC-HWZHA warning partitioning
- `unique: true` natural-key enforcement (`checkUniqueProperties`)
- state-machine transition enforcement (`Transitions.EnforceUpdate`)
- audit log entry (`recordEntityAudit`)

This contradicts root `CLAUDE.md`: *"All writes go through
`entitymanager.Manager`; do not write to `store.Store` directly from a write
path."*

## Trigger path

`internal/autocascade/runner.go:182-191` — for entities **created during a
cascade**, automation-set properties are applied and then re-written via the
host:

```go
for prop, val := range newAutoResult.PropertiesSet {
    created.SetString(prop, val)
}
// Re-write entity with updated properties.
if err := host.WriteEntity(ctx, created); err != nil {
```

So a metamodel automation that creates an entity and sets properties on it
during a cascade lands those values in the store unvalidated and unaudited.

## Why this is a real inconsistency, not a deliberate design

The sibling **top-level create path** explicitly does the opposite. At
`internal/entitymanager/manager.go:456-476`, after automation mutates the
created entity, it **re-runs `checkUniqueProperties` against the post-automation
values**, with the stated reasoning that the create path *"must not be the
weaker one."* The cascade's re-write has no equivalent.

Only the missing audit is documented (`cascadehost.go:76-78`). The skipped ACL,
validation, unique, and transition checks are undocumented — which suggests
oversight rather than an accepted trade-off.

## Suspected impact

- An automation can drive an entity into a state the state machine forbids.
- A `unique:` constraint can be violated by a cascade-created entity.
- A cascade write produces **no audit record**, so an entity can change with no trace
— notable given `audit-log` is a stable concept and the attribution work
(TKT-ZIRMGM) assumes manager-boundary coverage.
- Property values that would fail validation persist silently.

Severity is provisionally **high** because it is an unaudited, unvalidated write
path, but real-world reachability depends on how many projects use
`create_entity` automations with `set` actions inside a cascade — **needs
confirmation during analysis** before the priority is trusted.

## Relationship to TKT-80EWGM

Found while investigating TKT-80EWGM (unified `PatchEntity` primitive). It is
**explicitly out of scope** there: `PatchEntity` neither fixes nor worsens this.
Filed separately because routing `cascadeHost.WriteEntity` through the manager
changes cascade semantics — validation or a transition guard could now reject an
automation write **mid-cascade**, which needs its own design (partial-cascade
rollback is a known open question; cf. RR-KNXFF "Partial-failure rollback on
multi-op writes", wont-fix).

## Suggested analysis starting points

- Confirm reachability with a failing test: an automation that creates an entity and
sets a property violating a `unique:` constraint or a state-machine transition.
- Establish whether the audit gap is observable end-to-end (write happens, no audit
line).
- Decide the failure mode when a mid-cascade write is rejected — abort the cascade,
or collect into `autocascade.Outcome.Errors`? Note TKT-MSR8 ("Propagate
cascade-step warnings through autocascade.Outcome") is adjacent and may want
doing together.
- 5-whys should reach why the cascade host was given a raw store handle at all, rather
than the `gated()` mutator the scripted path already uses
(`autocascade/mutator.go:27-33`, wired at `manager.go:487` and
`manager.go:643`).
