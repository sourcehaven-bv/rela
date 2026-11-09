---
id: TKT-H91N91L
type: ticket
title: Peer identity and credentials
kind: enhancement
priority: medium
effort: m
started: "2026-11-08"
completed: "2026-11-08"
status: done
---

> **Superseded (TKT-TB2B8N5).** The credential part of this ticket (peer keys,
> the reserved `peer:` namespace, the route allowlist and `rela peer key
> create`) was removed. DEC-QKLMMD4 replaces DEC-81161C3: rela issues no
> credentials of its own, and a sync client is an ordinary principal that
> authenticates at the identity edge.

## Description

A peer instance needs an identity on the source instance.

**Corrected (issue #2119).** The first version of this ticket prescribed a
`system:<peer>` principal with a token from the token store. That cannot work:
`system:` is refused on every request path, and `internal/tokenstore` holds
outgoing OAuth tokens and authenticates no incoming request. Loosening the
namespace guard is not acceptable. DEC-81161C3 records the chosen mechanism and
the options compared.

- The peer authenticates with a **peer key**, a random bearer credential that
the source verifies at the request edge.
- A valid key stamps `peer:<name>`. This is an ordinary grantable principal, not
`system:`. `peer:` is reserved like `integration:`, so no header, environment
value or JWT subject can assert it.
- The source stores only the SHA-256 digest of each key, as
`peer_key.<name>: sha256:<hex>` in `.rela/secrets.yaml`. It re-reads the file at
most every 10 seconds.
- The principal goes through the existing identity edge (`access.Opener`): the
member-of walk and one `acl.Request`. The `principal_property` lookup never
applies to a `peer:` name. Roles come from `acl.yaml` as for any principal. The
principal type is `peer`, so a baseline can cap all peers.
- A peer key is read-only, through a route allowlist: `GET`/`HEAD` on
`/api/v1/` paths with no `_` segment, `_schema`, `_openapi.json`, and `POST
/api/v1/_reconcile`. Anything else answers 403.
- A bad key answers 401 and never falls through to another identity source.
- `rela peer key create <name>` generates a key. The client reads its key from
an environment variable, a file, or a named secret in `.rela/secrets.yaml`
(`rela reconcile diff --token-secret`). Never from a config file.
- The reconcile admission gate is keyed on the principal, so each peer has its
own bucket (RR-KT1PVVG).

Writes on the receiving side (TKT-I8L1EQU) run as a local job and keep a
`system:<peer>` principal for audit. That principal is never a request identity.

## Acceptance

- A peer key reads only what its role allows, through `_reconcile` and the list
endpoints.
- A wrong, revoked or malformed key answers 401 in every identity mode.
- A peer key cannot write.
- A request cannot assert a `peer:` name through the header, the environment
or a JWT.
- No key value appears in logs, config or the audit log.
