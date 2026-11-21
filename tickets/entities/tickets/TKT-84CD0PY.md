---
id: TKT-84CD0PY
type: ticket
title: Tenant API for an external provisioning job
kind: enhancement
priority: medium
effort: m
started: "2026-11-21"
completed: "2026-11-21"
status: done
---

## Description

A multi-tenant server creates an org's tenant only on an `organization.created`
webhook or, with lazy provisioning, on the org's first request. The first user
then waits on the setting-up screen. An external provisioning job has no way to
create the tenant beforehand or to read whether it is ready.

Add a host-level tenant API: `PUT /api/v1/_tenants/<org_id>` creates the tenant
through the existing provisioning runner, and `GET` reads its status. It is for
an operator or integration, not for end users. It reuses the inbound operator
auth that the IdP webhook uses: an ES256 JWT of the identity issuer, verified
against its JWKS, with its own audience (`-tenant-api-audience`). The token's
`org_id` must name the org in the path.

## Acceptance Criteria

- [x] `PUT` creates a tenant and answers 201, 200 when already active, 202 while running.
- [x] `GET` answers the status (`active`, `provisioning`, `suspended`, `erased`) or 404.
- [x] Only a verified token with the tenant API audience and a matching `org_id` is accepted; an end user's assertion or a webhook token is refused.
- [x] The server refuses to start when the audience equals the identity or webhook audience, or provisioning is off.
- [x] A create shares the rate limit, running cap and failure holds of the runner.
- [x] The API and its auth are documented.
