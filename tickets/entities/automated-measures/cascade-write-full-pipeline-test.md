---
id: cascade-write-full-pipeline-test
type: automated-measure
title: 'Test: cascade-created entity writes run the full manager pipeline (validation, unique, transitions, audit)'
description: 'Pins that the cascade re-write of an automation-created entity (autocascade/runner.go:182-191 -> cascadeHost.WriteEntity) runs the full manager pipeline: metamodel validation, unique-constraint checks, state-machine transition enforcement, and audit. Fails if cascadeHost.WriteEntity writes directly to store.Store. Behavioural, not structural, so a raw handle reintroduced by another route still fails.'
kind: test
location: internal/entitymanager/ (new test alongside audit_durability_test.go) — to be created by BUG-KIMZRK
status: proposed
---

## What it asserts

A write performed **during a cascade** — specifically the re-write of a
cascade-created entity after automation `set` actions
(`internal/autocascade/runner.go:182-191` → `Host.WriteEntity`) — is subject to
the same write pipeline as a top-level write:

1. metamodel validation runs (an invalid post-automation value does not persist
silently)
2. `unique: true` constraints are enforced against post-automation values
3. state-machine transitions are enforced (an automation cannot drive an entity into
a state the metamodel forbids)
4. an audit record is emitted for the cascade write

## Why it is needed

`cascadeHost.WriteEntity` (`internal/entitymanager/cascadehost.go:79-84`)
currently calls `h.deps.Store.UpdateEntity` directly, skipping all four. The
sibling top-level create path (`manager.go:456-476`) explicitly re-checks
uniques post-automation with the reasoning that it *"must not be the weaker
one"* — this measure pins that the cascade path is not weaker either.

## Shape

An integration test driving a real automation that creates an entity and sets a
property inside a cascade, with table cases for: a unique-constraint violation,
a forbidden state-machine transition, an invalid property value, and an
assertion that the audit sink received a record for the cascade write.

Prefer a **behavioural** assertion over a structural one (do not merely assert
`cascadeHost` holds no `store.Store` field) so a future refactor that
re-introduces a raw handle by another route still fails. This mirrors the
reasoning recorded in `AM-ungated-read-contract-not-identity` — assert the
contract, not pointer identity.

Complements `audit-durable-write-before-cascade-test`, which pins audit for the
*trigger* entity's durable write; this one covers entities written *inside* the
cascade.
