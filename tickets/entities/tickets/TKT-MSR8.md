---
id: TKT-MSR8
type: ticket
title: Propagate cascade-step warnings through autocascade.Outcome
kind: enhancement
priority: medium
effort: s
status: backlog
---

## Description

When a cascade creates an entity, that entity's soft-validation warnings are lost. `autocascade.Host.CreateEntity` returns only `(*entity.Entity, error)`, and `cascadeHost.CreateEntity` discards them. `Outcome.Warnings` already reaches the caller as `AutomationWarnings`.

Widen `Host.CreateEntity` to return the warnings and append them to `Outcome.Warnings`, prefixed with the created entity's id.

## Original description (2026-11-27: superseded by the text above)

## Summary

`autocascade.Host.CreateEntity` returns `(*entity.Entity, error)`. When the
cascade-driven entity has soft-validation warnings (per DEC-HWZHA),
`entitymanager.cascadeHost.CreateEntity` discards them because the Host contract
has no warning channel. Today this means warnings from cascade-spawned entities
never reach the caller.

Surfaced by architect review on TKT-IU2S (#6).

## Approach

Extend `autocascade.Outcome` with `Warnings []autocascade.Warning` (a tiny
three-field struct mirroring `entitymanager.Warning`). Widen `Host.CreateEntity`
to return `(*entity.Entity, []Warning, error)`. Manager translates
`autocascade.Warning` to `entitymanager.Warning` when merging Outcome into
`CreateResult.Warnings` / `UpdateResult.Warnings`.

## Out of scope

- Deeper Outcome restructuring (entities-deleted, relations-deleted
tracking).
