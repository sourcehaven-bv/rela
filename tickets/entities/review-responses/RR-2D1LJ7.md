---
id: RR-2D1LJ7
type: review-response
title: No client can send the face key on create
finding: 'The create body accepts a face key but no client populates it: the SPA CreateEntity type (frontend/src/types/entity.ts:285), sync client.go:255, MCP, CLI, Lua and webhooks all lack the field. On a type declaring faces: the server now demands a face and returns 422 face_required, which the SPA cannot satisfy.'
severity: critical
reason: 'Accurate but not shipping-blocking: no shipped or dogfooded schema declares faces:, only the worlds and perf prototypes. Faceless types are unaffected (a face is refused, not required). Filed as TKT-2RQMV4 with the client-side work, which is a frontend UX decision (face picker vs inherit-from-view) rather than part of this write-path fix.'
status: deferred
---

## Finding

The HTTP create body accepts a `face` key. No client sends one:

- SPA: `frontend/src/types/entity.ts:285` — `CreateEntity` has no `face` field
- sync: `client.go:255`
- MCP, CLI, Lua, webhooks: no field either

Since this change makes a face mandatory on a faced type, a SPA create against
such a type is a dead end: `write_handler.go:285` returns 422 `face_required`
and the client cannot express the value.

## Why deferred rather than fixed here

Verified against the tree: no shipped or dogfooded schema declares `faces:`.
Only `prototypes/worlds/project/schema.yaml` and
`prototypes/perf/project/schema.yaml` do. The `tickets/` and `docs-project/`
schemas are faceless, and a faceless type creates exactly as before — a face is
refused there, not required.

So the gap is real but reachable only from the worlds prototype, which is the
next piece of work rather than something in use today.

The remaining work is also a frontend design decision (explicit face picker
versus inheriting the current view's face) rather than a continuation of the
write-path fix, which argues for a separate ticket over widening this one.

Filed as **TKT-2RQMV4**.
