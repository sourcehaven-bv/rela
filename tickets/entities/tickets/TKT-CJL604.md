---
id: TKT-CJL604
type: ticket
title: Real-time co-editing of entity bodies (single node)
kind: enhancement
priority: medium
effort: l
status: backlog
---

## Description

Real-time co-editing of an entity's markdown body in the data-entry SPA, on a
single server process (DEC-OHJEKG, RES-L4FVT0 Option A, phase 1).

## Scope

- WebSocket endpoint `/api/v1/_collab/{type}/{id}` using `ygo`
(`provider/websocket`), one room per entity, behind the existing middleware.
Join requires read access (hidden entity = 404); a principal without update
permission joins read-only through `Server.Authorize`.
- `statsResponseWriter` gains a `Hijack` forwarder (gorilla/websocket does not
follow `Unwrap`).
- Server-assigned seeder: the first joiner seeds the room from the stored
markdown; other joiners wait for `synced`.
- Collab mode in `MilkdownEditor`/`useAutoSave`: the shared document is the
authority; the editor ignores server echoes of content; every client keeps
autosaving through PATCH (identical content, idempotent).
- Build-version check in the handshake; a stale SPA reloads.
- Provider with `disableBc: true`; presence and cursors through awareness.
- Editor fixes found by the spike: y-prosemirror node-marks patch
(`patch-package`, offered upstream) and `alignment` default `null` on GFM cells.
`@milkdown/plugin-collab` pinned to the `@milkdown/kit` version.
- The editor corpus test also runs in collab mode.
- Desktop: collab disabled. Postgres multi-node: document sticky routing by
URL path.

## Out of scope

External-edit merge (phase 2), cross-node relay (phase 3), properties.
