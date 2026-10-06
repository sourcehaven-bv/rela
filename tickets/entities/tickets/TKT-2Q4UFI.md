---
id: TKT-2Q4UFI
type: ticket
title: Automation action that enqueues a Lua script as a background job
kind: enhancement
priority: medium
effort: m
status: ready
---

## Description

Add an automation action that enqueues a Lua script as a background job on
`jobs.Queue`, instead of running it inline on the write path. An integration
push calls an external API; inline, every save would wait on that API and could
run slow I/O inside a `store.Store.Tx` (FEAT-XYQMUB).

## Requirements

- The job becomes runnable only after the triggering write commits
(`jobs.WithDeferral`).
- Coalesced per entity and script through an `IdempotencyKey`, so several
quick edits cause one run.
- Retry uses the existing intent enum; no per-call knobs.
- The script runs with the identity and capabilities the automation declares,
like scheduled tasks.
