---
id: TKT-PEWBZ8
type: ticket
title: 'Proxied deployments: --allowed-host flag for requireLocalHost (and the hardcoded http:// origin)'
kind: enhancement
priority: medium
effort: s
status: backlog
---

## Description

`requireLocalHost` (middleware_security.go:113) has **no path exemption and no
flag**, so behind a reverse proxy any request carrying a forwarded `Host` is
rejected with 403 before it reaches a handler.

This is a **general proxy-deployment gap**, not a CalDAV one. It already affects
the REST API, the SPA and the ICS feeds today; the CalDAV work (TKT-N8RESF)
merely surfaced it, and it was split out of that ticket so it is described in
the terms that actually apply.

## The current behaviour

`newSecurity` (middleware_security.go:52-102) builds the allowlist from the bind
address:

- **Loopback bind** (the default): exactly `127.0.0.1:PORT`, `localhost:PORT`,
`[::1]:PORT`. A proxied or LAN request with any other `Host` → 403
`host_not_allowed`.
- **Specific non-loopback bind**: only that exact `host:port`. So
`--bind 192.168.1.5` accepts `Host: 192.168.1.5:8080` but not `Host:
rela.example.com`.
- **`--bind 0.0.0.0`**: `allowedHosts` is set to **nil**, which disables the
Host check entirely.

`--allowed-origin` exists but feeds `allowedOrigins` only, **not**
`allowedHosts`.

So the only way to accept a proxied `Host` today is `--bind 0.0.0.0`, which
turns the check off wholesale. That is too blunt: an operator who wants to
accept one hostname has to accept all of them.

## Deliverables

1. **`--allowed-host` flag** (repeatable, or comma-separated — match
`--allowed-origin`'s existing shape) adding entries to `allowedHosts` without
disabling the check. Unlisted hosts must still 403.
2. **Document the interaction with `--bind 0.0.0.0`**, which currently nils the
allowlist. Decide during planning whether supplying `--allowed-host` alongside
it should re-enable enforcement — arguably yes, since the operator has stated
the hosts they expect, but it changes an existing behaviour and should be a
deliberate call rather than a side effect.
3. **Address the hardcoded `"http://"` origin derivation**
(middleware_security.go:80-90). `newSecurity` builds `allowedOrigins` with a
literal `http://` scheme, so a deployment that later terminates TLS *in rela*
would reject every same-origin browser request as `origin_not_allowed`. Latent
today (rela runs plaintext behind a proxy), so a comment naming the trap is an
acceptable minimum; a `TLS bool` on `SecurityConfig` is the real fix if cheap.

## Acceptance criteria

1. `--allowed-host rela.example.com` lets a proxied request through while an
unlisted host still gets 403 `host_not_allowed`.
2. The flag composes with the existing bind-derived defaults rather than
replacing them (loopback hosts keep working).
3. The `--bind 0.0.0.0` interaction is tested and documented, whichever way it
is decided.
4. The hardcoded-`http://` trap is fixed or commented at the site.
5. `docs/server-security.md` covers the flag alongside `--allowed-origin`.

## Why this is worth doing properly

The current workaround (`--bind 0.0.0.0`) is a security downgrade dressed as a
deployment step: it silently removes a control rather than configuring it. An
operator following a proxy guide would do it without realising the Host check
went away — which is exactly the kind of "documented workaround becomes the
default posture" problem worth closing while it is still small.
