---
id: TKT-7QM4RB
type: ticket
title: 'Carry allow_acl_bypass through the declarative create_relation: automation action'
kind: enhancement
status: backlog
priority: medium
effort: m
---

## Description

A trigger's `create_relation:` action cannot declare `allow_acl_bypass`. The Lua action already carries it on `triggers.Action.AllowACLBypass`. But `triggers.RelationToCreate` (`internal/flowcfg/triggers/types.go`) holds only the relation and the trigger name. `autocascade.Runner.applyRelationCreations` passes that relation to `Host.WriteRelation` and nothing else.

Add the elevation to `RelationToCreate` as a bare string, since autocascade must not import metamodel. Pass it to `WriteRelation` through an options struct, not a second method. Nothing reads the value until TKT-M3W8PK lands.

## Original description (2026-11-27: superseded by the text above)

## Why

`AllowACLBypass` (`metamodel/types.go:646-656`) is the codebase's declared
elevation pattern: operator-only, an enum since TKT-Y3JVFK
(`read`/`write`/`read+write`), closure-scoped, audited, real principal
preserved. Its godoc says it is **"Ignored for non-Lua actions."**

That asymmetry is a problem on its own, and it is a **hard prerequisite** for
gating automation relation writes (TKT-M3W8PK): without it, that gate is a
breaking change with no operator opt-out.

## The gap

The declarative `create_relation:` action carries **no action provenance** to
the write:

- `AllowACLBypass` reaches Lua only via `automation.LuaToExecute`
  (`automation/types.go:43,137` → `script/luascriptrunner.go:164`).
- The declarative action emits `automation.Result.RelationsToCreate
  []*entity.Relation` (`automation/types.go:146`) — **a bare relation slice**.
- `Runner.applyRelationCreations(ctx, host, triggerEntity, relations, outcome)`
  (`autocascade/runner.go:209-213`) receives only that slice.

By the time `Host.WriteRelation` is called, which action emitted the relation is
unrecoverable.

## Scope

Thread per-action elevation from `metamodel.AutomationAction` →
`automation.Result` → `autocascade.Runner` → `Host.WriteRelation`.

- Change `Result.RelationsToCreate` from `[]*entity.Relation` to a struct
  carrying the relation plus its elevation.
- Change the `autocascade.Host.WriteRelation` **consumer-side interface**
  (`autocascade/host.go:65`). Use an options struct on the call — **not** a
  second `WriteRelationElevated` method.
- `internal/autocascade` may not import `metamodel` (`runner.go:278`), so the
  value crosses as a bare string, exactly as `ScriptAction.AllowACLBypass`
  already does (`scriptrunner.go:73`).

Behaviour is unchanged until TKT-M3W8PK lands: nothing yet consults the value on
this path. This ticket is pure plumbing plus tests that the value arrives.

## Out of scope

- Gating the write (TKT-M3W8PK).
- `create_entity:` elevation.

## Files

`internal/automation/types.go`, `internal/automation/engine.go`,
`internal/autocascade/host.go`, `internal/autocascade/runner.go`,
`internal/entitymanager/cascadehost.go`.
