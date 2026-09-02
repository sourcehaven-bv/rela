---
id: TKT-KX0DPZ
type: ticket
title: 'idp-sync example: validate webhook claims before interpolating them'
kind: docs
priority: low
effort: xs
status: done
---

## Description

`examples/idp-sync.lua` interpolates `org_id` and `user_id` — taken from the
verified webhook JWT — straight into an outbound request path and an entity
query with no validation:

```lua
local path = "/api/v1/orgs/" .. org .. "/members/" .. uid
local found = rela.list_entities("person", "sub=" .. uid)
```

GitHub issue #1083 (IB-review rela#1069). Severity: low — the JWT is
cryptographically verified (ES256, with a confused-deputy guard via a separate
audience), and this is an **example** script, so the blast radius is whoever
adapts it.

But that is exactly why it matters: an example is copied. A compromised or
misconfigured IdP emitting a `sub` containing `/`, `?`, `#` or a newline would
reshape the request path or the filter, and anyone who reused the script
inherits that.

## Fix

An allowlist regex on both identifiers before interpolation, with a comment
explaining why it is there and how to widen it safely — including that some IdPs
(Auth0's `auth0|abc123`) issue subjects the default set rejects.
