---
id: TKT-QTNX
type: ticket
title: Define entitymanager.Manager (real implementation, not adapter)
kind: refactor
priority: high
effort: m
status: done
---

## Summary

Today `internal/entitymanager` has only the interface and result types; the real
implementation is `wsEntityManager` in `internal/workspace/manager.go`, which
adapts to Workspace's "legacy write API" (`createEntity`, `updateEntity`, etc.
private methods on Workspace).

Build a real `entitymanager.Manager` type with explicit, typed dependencies,
replacing the wsEntityManager indirection.

## In scope

- New `entitymanager.Manager` struct with typed `Deps`:
  - `Store store.Store` — persistence
  - `Meta *metamodel.Metamodel` — schema
  - `Cascade *automation.Runner` — automation orchestrator (depends on the Runner extraction ticket landing first)
  - `Validator validator.Validator` — write-time validation
- Methods implementing `entitymanager.EntityManager` (CreateEntity, UpdateEntity, DeleteEntity, RenameEntity, CreateRelation, UpdateRelation, DeleteRelation), each running the standard write pipeline (validate → apply property changes from cascade → store write → audit hook on completion → cascade for derived structural actions).
- A shared internal `run(ctx, op writeOp) (*Result, error)` helper that runs the pipeline so each method body is short and so adding a new pipeline step (audit, policy) is one place.
- Constructor `entitymanager.New(deps Deps) (*Manager, error)` validates required fields are non-nil.
- Manager implements `automation.Host` structurally (the `Runner` calls back into `m.CreateEntity` / `m.GetEntity` / `m.CreateRelation`).

## Out of scope

- Wiring Manager into production (separate per-command tickets).
- Audit, principal, policy fields. Those are added by their respective tickets when they land.
- Removing `wsEntityManager` adapter immediately. It can stay as a fallback during the migration; remove once all consumers are off it.

## Depends on

- automation.Runner extraction (separate ticket, must land first).

## Risks

- The pipeline shape (`validate → cascade-properties → store-write → audit → cascade-structural`) needs to match Workspace's current sequencing exactly, or behavior changes silently. Audit the existing Workspace code carefully and write tests that pin the order.
- Validation timing: today some validation happens before the write, some during automation processing. Decide whether Manager's pipeline preserves that split or normalizes it; document the choice.
