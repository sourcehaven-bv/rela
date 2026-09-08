---
id: TKT-NU247U
type: ticket
title: MCP server hot-reloads schema.yaml on change
kind: enhancement
priority: medium
effort: m
status: done
---

## Problem

The MCP server loads `schema.yaml` once at startup. The metamodel is baked into
`mcp.Deps.Meta` **and** deep into every derived service built by
`appbuild.assemble` — validator, entitymanager (automations, transitions,
computed, templater), affordances, field redactor.

So editing `schema.yaml` during an MCP session requires a manual server restart
before a new entity type, enum value, relation target or validation rule takes
effect. Iterative modelling in an agent session is interrupted by a restart
cycle on every schema edit.

Upstream: sourcehaven-bv/rela#644.

## Why reloading only `Deps.Meta` is not enough

`create_entity` validation, enum checks and relation-target checks all run
through `EntityManager`, which holds its own `*metamodel.Metamodel`. Swapping
only `Deps.Meta` would refresh the read surfaces (`get_schema`, resources,
prompts) while leaving writes validating against the stale schema — the exact
symptom the issue reports, only harder to diagnose because part of the surface
would appear to update.

## Approach

`appbuild.SharedBase` already splits the tenant-independent half (config,
options, acl.yaml, metamodel) from the per-store half. `NewSharedBase` re-reads
`schema.yaml`; `SharedBase.Assemble(store, searcher, visible, closer)` rebuilds
everything metamodel-derived against an **existing** store — no store close, no
search reindex.

The obstacle: each `Assemble` also starts per-assembly background services (job
queue with its worker pool, mail outbox worker, data-migration GC sweep, and on
postgres a version sweep), and the only teardown — `Services.Close` — *also*
closes the shared store and search closer. Re-assembling per schema change would
leak a job queue per edit.

So this ticket adds a narrowly-scoped teardown that stops the per-assembly
background services while leaving the shared store/searcher untouched, and the
MCP wiring drives it from a `schema.yaml` watcher.

## Constraints

- A parse error must retain the last-good metamodel and log clearly — never
leave the server without a schema.
- Readers observe either the pre- or post-reload snapshot, never a torn one
(`atomic.Pointer`, per the CLAUDE.md state-publish rule).
- The reload must not close or reindex the store.
