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
- A read-only connection may not send a BroadcastStateless frame to other
  peers. Upstream dropped only its document and awareness writes.
- An in-band auth frame (`OnTokenAuth`) may replace a connection's stamp but
  never remove it.
- Import paths point at this directory. Upstream's Redis cluster test is
  left out, because rela does not depend on Redis.

Each change is marked `rela fork` in the code.

## Keeping the fork current

`govulncheck`, osv-scanner and Dependabot see the `github.com/reearth/ygo`
version in `go.mod`, not this fork. Two checks keep the fork visible
(TKT-IBT44K5, issue #2105).

- **Upstream release.** `UpstreamVersion` in `upstream.go` records the
  release the fork was taken from. `TestUpstreamVersion` fails when `go.mod`
  requires another ygo version. A Dependabot bump of ygo therefore fails CI
  and is not auto-merged.
- **Advisory.** The `ygows-advisories` job in
  `.github/workflows/security.yml` runs daily. It asks OSV for advisories on
  ygo at `UpstreamVersion` and keeps those that name an import path in a
  forked package (`provider/websocket`, `internal/relaylane`,
  `internal/roomname`, `internal/updatecheck`) or name no path at all. If
  any remain, it opens or updates the issue "ygows fork: OSV advisory on the
  forked ygo relay". The first clean run closes it.

### Re-syncing after an upstream release

1. Diff upstream `provider/websocket` and the three internal packages
   between `UpstreamVersion` and the new release.
2. Apply the upstream changes here, and keep every `rela fork` change listed
   above. Update this README if a change is no longer needed.
3. Set `UpstreamVersion` to the new release, in the same commit as the
   `go.mod` bump.
4. Run `go test -race ./third_party/ygows/...` and the co-editing tests in
   `internal/dataentry`.

### Triaging an advisory

- If upstream has fixed it, re-sync with the fixed release as above. The
  next run then queries the new version and closes the issue.
- If the fork is not affected, or the fix was ported here by hand, add the
  OSV id and the reason to `osv-ignore.txt`. Remove the entry once
  `UpstreamVersion` is past the affected range.

`scripts/ygows-advisories.sh` holds the filter, and
`scripts/ygows-advisories-test.sh` tests it in CI.
