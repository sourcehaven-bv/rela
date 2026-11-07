---
id: TKT-8D1I8PA
type: ticket
title: 'tenant: choose the tenant for anonymous requests (or decide there is none)'
kind: enhancement
priority: low
effort: s
status: backlog
---

## Description

Pratique can forward requests on `upstream.anonymous_paths` with no assertion
and an `X-Pratique-Anonymous: true` marker. Those requests carry no `org_id`. In
tenant mode rela fails closed on a missing org, so anonymous requests reach no
tenant's data.

That is correct until a tenant needs public content: a public portal, a shared
read-only link, a signup form. Then rela needs another way to pick the tenant.

## Options to evaluate

- Host to org mapping, for example from Pratique's verified org domains,
recorded in the tenant map.
- An explicit org slug in the path.
- No anonymous tenant access at all (the current behaviour), documented.

## Constraints

- The tenant chosen for an anonymous request must never come from a value the
caller can set freely without the operator having bound it to that tenant.
- An anonymous request still goes through the ACL as an anonymous principal
within the chosen tenant.

## Acceptance criteria

1. A decision record states which option, and why.
2. If an option other than "none" is chosen, a test shows an anonymous request
on tenant A's host cannot read tenant B's public content.
