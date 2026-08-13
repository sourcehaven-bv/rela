---
id: cascade-write-full-pipeline-test
type: automated-measure
title: 'Test: cascade-created entity re-checks validation/unique/transitions against POST-automation values'
description: 'Pins that automation-set properties applied to a cascade-created entity (autocascade/runner.go:181-187 -> cascadeHost.WriteEntity) are re-validated before persisting: unique-constraint checks, metamodel validation, and state-machine enforcement against the POST-automation values. Mirrors the top-level create path (manager.go:456-476), which already re-checks uniques post-automation on the stated grounds that the create path ''must not be the weaker one''. NOTE: deliberately does NOT assert audit — the cascade create is already audited via recordCascade (cascadehost.go:62), and the absence of a second record on the property-set step is intentional (it would double-count one creation).'
kind: test
location: internal/entitymanager/ (new test alongside audit_durability_test.go) — to be created by BUG-KIMZRK
status: proposed
---

## What it asserts

Automation-set properties applied to a **cascade-created** entity
(`internal/autocascade/runner.go:181-187` → `Host.WriteEntity`) are re-checked
against their **post-automation** values before being persisted:

1. `unique: true` constraints are enforced against post-automation values
2. metamodel validation runs (an invalid automation-set value does not persist
silently)
3. state-machine entry is enforced (an automation cannot drive a
cascade-created entity into a state the metamodel forbids)

## What it deliberately does NOT assert

**Audit.** The cascade create is already audited — `cascadehost.go:62` calls
`recordCascade(ctx, audit.OpCreateEntity, ...)`. The absence of a *second*
record on the property-set step is intentional and documented
(`cascadehost.go:76-78`): a second record would double-count one creation.
Asserting an audit record here would pin a false expectation and fail against
correct code.

An earlier draft of this measure did assert it, because BUG-KIMZRK was filed
with an overstated reachability claim. Corrected 2026-08-12.

## Why it is needed

`createCore` runs its checks against the **pre-automation** candidate;
`runner.go:181-187` then mutates the entity with `PropertiesSet` and calls
`WriteEntity`, which persists via `Store.UpdateEntity` with no re-check.

The asymmetry is the defect: the top-level create path (`manager.go:456-476`)
explicitly re-runs `checkUniqueProperties` against post-automation values, on
the stated grounds that the create path *"must not be the weaker one."* The
cascade path has no equivalent.

## Shape

Integration test with a real automation that creates an entity inside a cascade
and sets a property on it, table-driven over: a unique-constraint violation, an
invalid property value, and a forbidden state-machine entry value.

Assert **behaviourally** (the write is rejected / the bad value does not land),
not structurally — do not assert that `cascadeHost` holds no `store.Store`
field, or a future refactor reintroducing a raw handle by another route would
still pass. Same reasoning as `AM-ungated-read-contract-not-identity`.
