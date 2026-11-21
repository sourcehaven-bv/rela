---
id: TKT-59HWQ76
type: ticket
title: Tenant management API behind the edge
kind: enhancement
priority: high
effort: l
started: "2026-11-21"
completed: "2026-11-21"
status: done
---

## Description

harbourmaster (the SaaS control plane) must manage the tenants of a multi-tenant
rela-server from billing events. The tenant API from TKT-84CD0PY verifies its
own IdP token (`-tenant-api-audience`), but nothing can mint that token, and it
only creates and reads.

Replace its token check with the edge model: pratique authenticates
harbourmaster as an operator principal and forwards an edge assertion with
`principal_type=operator`, no `org_id` and a role. The server authorizes the API
on a configured role (`-tenant-api-role`), as it trusts the edge for users.
Extend the API with state (active, read_only, suspended), plan and entitlements,
archive, and usage.

## Acceptance Criteria

- [ ] The API accepts only a verified edge assertion with principal_type operator, no org_id and the configured role; a user assertion or a missing one is refused.
- [ ] PATCH sets state (active, read_only, suspended) and plan/entitlements; a tenant an operator or the IdP suspended is not resumed by the API.
- [ ] A read_only tenant serves reads and exports and refuses writes with 403 tenant_read_only.
- [ ] DELETE archives: the data stays, access is refused, and only an operator brings it back or erases it.
- [ ] GET usage reports users, active users and storage bytes.
- [ ] Plan and entitlements are stored in the control table and exposed on GET /api/v1/_tenant.
- [ ] The design is documented in docs/postgres-backend.md.
