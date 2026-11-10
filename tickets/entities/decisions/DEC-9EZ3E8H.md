---
id: DEC-9EZ3E8H
type: decision
title: No anonymous tenant access in tenant mode
context: TKT-8D1I8PA asked how tenant mode should pick a tenant for anonymous requests. Pratique can forward anonymous_paths requests with no assertion and no org_id; rela fails closed on a missing org. Options were host-to-org mapping, an org slug in the path, or no anonymous tenant access.
consequences: Tenant mode keeps failing closed when org_id is missing. Every network deployment runs behind Pratique with a verified ES256 JWT carrying org_id; IdP webhooks are signed and name their org; local single-store use has no tenants. Public content belongs in Pratique or a separate publishing service. Pratique anonymous_paths requests reach no tenant. A shared read-only link would be a tenant-bound signed token in its own ticket.
date: "2026-11-10"
status: accepted
---

## Decision

Tenant mode has no anonymous tenant access. A request without a verified
`org_id` reaches no tenant. This is the current fail-closed behaviour; it is now
the intended behaviour.

## Reasons

- Every network deployment of rela runs behind Pratique. Every request carries
a verified ES256 JWT with `org_id`.
- IdP webhooks are not anonymous. They are authenticated by signature and name
their organization.
- Local (single-store) use has no tenants, so the question does not arise.
- Public content (signup, onboarding, a public portal, CMS-style publishing)
belongs in Pratique or a separate publishing service, never in rela.

## Consequences

- Requests on Pratique `upstream.anonymous_paths` (marked
`X-Pratique-Anonymous: true`, no assertion) reach no tenant in tenant mode.
- Host-to-org mapping and org slugs in the path are rejected.
- A future shared read-only link would be a signed token bound to one tenant,
not an anonymous request. That needs its own ticket.

Closes TKT-8D1I8PA.
