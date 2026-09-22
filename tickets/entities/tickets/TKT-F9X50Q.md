---
id: TKT-F9X50Q
type: ticket
title: store.GraphQuery drops AllStates, so ACL-gated readers silently lose per-face scans
kind: refactor
priority: medium
effort: m
status: backlog
---

Surfaced by TKT-CICJSN's reviews (RR-16R183). Not a regression from that ticket
— it is a gap in the pushdown seam that TKT-CICJSN made visible.

`store.EntityQuery.AllStates` has no counterpart on `store.GraphQuery`
(`internal/store/graphquery.go`). `visibility.listPushdown` discards the
`EntityQuery` and composes a `GraphQuery` the moment a principal's grants
compile to a policy query (`internal/visibility/pushdown.go`, the `rqr.Query !=
nil` branch), so an `AllStates: true` request is dropped and `graphquerynaive`
collapses each id to one world prime.

The `AllowAll` branch copies the `EntityQuery`, so `AllStates` survives there.
Net effect: a privileged principal scans every face, a gated one scans the prime
only — the divergence the `World` and `FaceIn` fields were each added to this
type to prevent, for the same stated reason.

`grep -rn "AllStates" internal/visibility/ internal/acl/` finds nothing outside
tests: the path neither carries the flag nor rejects it.

**Fail-closed today**, which is why it is medium and not high: an unscanned face
means a finding is MISSED, never invented, and no row is disclosed that the gate
would have withheld.

Fix shape — either carry `AllStates` onto `GraphQuery` alongside `World`/
`FaceIn` at the same seam and for the same reason, or have pushdown yield
`ErrInvalidQuery` when it cannot honor the flag (mirroring the `ReadQueryFor`
error branch just above it). Prefer the former.

Needs a parity test in the shape of `internal/visibility/worldparity_test.go` —
the existing decorator/pushdown parity test did not catch this because nothing
exercises `AllStates` through a gate.

Belongs with TKT-O7R2A1 (the face-narrowing work on the same seam).
