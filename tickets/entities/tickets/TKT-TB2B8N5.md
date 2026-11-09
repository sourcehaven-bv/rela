---
id: TKT-TB2B8N5
type: ticket
title: Remove peer keys; machine clients authenticate at the identity edge
kind: refactor
priority: high
effort: m
started: "2026-11-08"
completed: "2026-11-09"
status: done
---

## Description

Remove the peer-key credential system that PR #2156 (TKT-H91N91L, DEC-81161C3)
added, and replace its decision.

The owner decided that rela must not have its own credentials for machine
clients. Identity belongs to the identity edge: the verified JWT from Pratique
(or another OIDC proxy), or the principal header. Pratique already issues
machine-client tokens. A sync peer is an ordinary principal: what it reads comes
from `acl.yaml`, as for any user. `_reconcile` is an API that any user may call,
and its reads are ACL gated.

## Scope

Remove:

- the `internal/peerkey` package;
- the peer-key check at the request edge (`internal/dataentry/peerkey_edge.go`
and its wiring in `router.go` and `cmd/rela-server/serve.go`);
- the reserved `peer:` namespace, `principal.ValidPeerName`, `TypePeer`;
- the peer route allowlist and the `peer_route_refused` error;
- the skip of the `principal_property` lookup for `peer:` names;
- `rela peer key create`;
- the `peer_key.*` secrets;
- the "Peer keys" docs sections;
- the tests for all of the above.

Keep:

- the docs fixes for #2140, RR-U6M954C and DOCS-JA3ZYX2;
- the rule that scripts cannot read credential secrets, with neutral naming;
- `rela reconcile diff --token-secret NAME`, with neutral naming;
`--token-env` and `--token-file` stay.

Records: a new decision supersedes DEC-81161C3; TKT-H91N91L and TKT-I8L1EQU get
notes. Issues #2157 and #2158 concern only removed code.
