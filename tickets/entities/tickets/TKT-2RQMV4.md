---
id: TKT-2RQMV4
type: ticket
title: 'SPA cannot create an entity on a faced type: CreateEntity has no face field'
kind: enhancement
priority: high
effort: s
status: backlog
---

## Problem

BUG-HC6I2T made the write path resolve and authorize an explicit face, and made
a face mandatory when the entity type declares `faces:`. A create that names no
face on a faced type is refused:

`internal/dataentry/write_handler.go:285` returns 422 `face_required`.

The HTTP API accepts a `face` key on the create body. No client sends one:

- SPA: `frontend/src/types/entity.ts:285` — `CreateEntity` has no `face` field.
- sync: `client.go:255`
- MCP, CLI, Lua, webhooks: no field either.

So on a faced type the SPA create flow is a dead end: the server demands a face
and the client cannot express one.

## Why this did not block the fix

No shipped or dogfooded schema declares `faces:` today — only
`prototypes/worlds/project/schema.yaml` and
`prototypes/perf/project/schema.yaml` do. The `tickets/` and `docs-project/`
schemas are faceless, and a faceless type still creates exactly as before (a
face is refused, not required).

The gap is therefore real but currently reachable only from the worlds
prototype, which is also the thing that needs it next.

## Fix

1. Add `face?: string` to `CreateEntity` and thread it from the create form.
`DynamicForm.vue` already resolves a face for rendering; the create call must
send the same value it rendered against.
2. Decide per client whether the face is user-choosable or inherited from the
current view's face. For the SPA the current face is the right default —
creating from a `@published` view should create the published face.
3. Sync/MCP/CLI/Lua: add the field where a create is expressible at all.

## Scope note

This is the client half of BUG-HC6I2T. That bug fixed the server: the authorized
face and the written face are now the same value. This ticket gives clients a
way to name that value.
