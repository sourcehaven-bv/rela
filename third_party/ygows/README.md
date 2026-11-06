# ygows

A fork of the WebSocket relay in
[ygo](https://github.com/reearth/ygo) `provider/websocket` v1.55.0, with
the internal packages it needs (`relaylane`, `roomname`, `updatecheck`). The
upstream `crdt`, `sync`, `awareness`, `encoding`, `persistence` and `cluster`
packages are still imported from `github.com/reearth/ygo`. The code is under
the upstream MIT licence in `LICENSE`.

rela forked the relay because the relay owns the WebSocket, and the host
needs to decide what each connection may put in its awareness state
(TKT-6NPNITP).

## Changes from upstream

- `ConnectionConfig.StampAwareness` rewrites every non-null awareness state
  a connection sends, before the room applies or forwards it
  (`presence.go`). rela uses it to label a cursor with the authenticated
  principal.
- A connection may not change or remove a live awareness client that another
  connection, or another node, answers for (`peer.go`, `ownerAdmits`).
  Presence relayed from another node through `cluster.Relay` is not stamped
  again: the trust boundary is that node, which stamped it on arrival.
- `ConnectionConfig.Owner` binds each awareness client id to the owner that
  first made it live, for the room's lifetime. Only connections with that
  owner may change or remove it, so a reconnecting tab keeps its cursor.
  A peer without an owner keeps the rule above.
- An in-band auth frame (`OnTokenAuth`) may replace a connection's stamp but
  never remove it.
- Import paths point at this directory. Upstream's Redis cluster test is
  left out, because rela does not depend on Redis.

Each change is marked `rela fork` in the code.
