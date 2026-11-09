---
id: DEC-QKLMMD4
type: decision
title: No rela-issued credentials; machine clients authenticate at the identity edge
context: 'DEC-81161C3 added peer keys: credentials that rela issued and verified itself for sync peers. The owner decided that identity belongs to the identity edge. That is the verified JWT from Pratique (or another OIDC proxy) or the principal header. Pratique already offers machine-client tokens, so rela does not need a credential system of its own.'
consequences: 'One identity source per mode again: the JWT gate in JWT mode, the principal header otherwise. peer: is no longer reserved and has no meaning. A sync client needs an identity at the edge (a machine token or a proxy user) and a role in acl.yaml. Header-mode callers without identity share one reconcile rate key. Clients keep --token-env, --token-file and --token-secret; remote_credential. secrets are never script readable. #2156 was in no release, so no deployment holds peer keys.'
date: "2026-11-08"
status: accepted
---

## Decision

rela issues no credentials of its own. Machine clients authenticate at the
identity edge, like people do:

- in JWT mode, by authenticating to Pratique or another OIDC proxy with a
machine-client credential (Pratique offers these); the proxy forwards a signed
assertion that rela verifies;
- in header mode, through the trusted proxy that sets the principal header.

A sync client is an ordinary principal. What it reads comes from `acl.yaml`, as
for any user, and `visible:` redacts its fields as for anyone else. `POST
/api/v1/_reconcile` is an API that any user may call; its reads are ACL gated,
and its rate gate is keyed on the resolved principal. In header mode the callers
without the identity header are all the user `unknown` and share one key. That
is intended.

The client side keeps reading a bearer token from an environment variable, a
file or a named secret (`rela reconcile diff --token-secret
remote_credential.<name>`). A secret under `remote_credential.` is never
readable by a script.

This replaces DEC-81161C3. The peer keys, the reserved `peer:` namespace, the
peer route allowlist and `rela peer key create` are removed (TKT-TB2B8N5).
