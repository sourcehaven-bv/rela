---
id: TKT-WEBI
type: ticket
title: 'data-entry: per-request Principal from HTTP header'
kind: enhancement
priority: medium
effort: s
status: in-progress
---

The data-entry server currently stamps every request with
Principal{User:"unknown", Tool:"data-entry"} via defaultPrincipalResolver
(internal/dataentry/router.go) — recording the server process owner for every
human web user would be misleading. The audit-log PR (#763) deliberately
introduced the PrincipalResolver func-type seam exactly so this follow-up could
plug a header-aware resolver in without restructuring middleware.

This ticket: add a resolver that reads Principal.User from a configurable header
(default `X-Forwarded-User`), with `$RELA_DATAENTRY_USER` env override for local
dev. Fall through to "unknown" when the header is absent. Document the trust
boundary — the header is only as trustworthy as the reverse proxy that sets it;
operators running data-entry on loopback without a proxy should rely on the env
override.

## In scope

- New `HeaderPrincipalResolver(headerName string) PrincipalResolver` constructor in `internal/dataentry/router.go`.
- Env override: `$RELA_DATAENTRY_USER` wins over the header (cheap dev escape hatch).
- Wiring: `cmd/rela-server/main.go` constructs the resolver from a flag/env (`--principal-header X-Forwarded-User`) and passes it to `NewRouter` via a small option / setter.
- Docs: trust-boundary warning in `docs-project/entities/guides/GUIDE-audit-log.md` (the data-entry section) and in `docs/security.md` (hand-written, not generated).

## Out of scope

- OAuth / OIDC integration (separate, larger ticket).
- Multi-user authorization (ACL). Principal is identity only; this PR doesn't introduce policy.
- Cookie / session storage. Header-based attribution is the minimal viable step for proxied deployments.

## Why now

The seam was prepared in the audit-log PR specifically to defer this. Closing it
leaves data-entry's audit attribution honest for proxied / SSO'd deployments.
Trivial to add (~30 LOC + tests).
