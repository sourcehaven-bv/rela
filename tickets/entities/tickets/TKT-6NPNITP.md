---
id: TKT-6NPNITP
type: ticket
title: Fork the ygo relay and stamp awareness identity on the server
kind: enhancement
priority: high
effort: m
started: "2026-11-05"
completed: "2026-11-06"
status: done
---

## Description

The co-editing relay (TKT-CJL604) forwards each editor's awareness state as the
browser sent it. The `user` field on a cursor is therefore client-chosen: an
editor can label their cursor with another user's id (PR guidance G-006).

ygo's websocket provider owns the socket and has no hook to vet or rewrite
awareness content per connection. Fork the relay (`provider/websocket` and the
internal packages it needs, MIT) into `third_party/ygows`, keep importing the
upstream `crdt`, `sync`, `awareness` and `cluster` packages, and change it so:

- each connection's awareness `user` is set by the server from the
authenticated principal (`ConnectionConfig`), overwriting what the client sent;
- a connection may not update a live awareness client owned by another
connection.

Fan-out between nodes keeps the upstream `cluster.Relay` seam; a Postgres
implementation is a follow-up, so Redis stays optional.

## Acceptance

- A client that sends another user's id in awareness is shown under its own id.
- A client cannot overwrite another connection's cursor state.
- The forked code keeps the upstream licence notice and its tests.
