---
id: TKT-2R2RW5H
type: ticket
title: 'IdP webhook: accept org-scoped events that carry no user_id'
kind: enhancement
priority: medium
effort: s
status: backlog
---

## Description

Pratique sends org lifecycle events (`organization.created`,
`organization.updated`, `organization.deleted`). These carry `org_id` and no
`user_id`. The IdP webhook receiver rejects any event without a `user_id` with a
400 (`internal/dataentry/webhook.go:131`), because today it only dispatches
membership events to a user-provisioning action.

TKT-TNPRV8 plans to provision tenants from `organization.created` and TKT-TNERAS
will act on `organization.deleted`. Both are blocked until the receiver accepts
org-scoped events.

## Approach

- Decide the required claims per event family: membership events need
`user_id` and `org_id`; organization events need `org_id` only.
- Route organization events to a separate handler seam that TKT-TNPRV8 and
TKT-TNERAS fill in. Until then, acknowledge and log them.
- Keep verification unchanged: separate `aud`, dedup on `jti`.

## Acceptance criteria

1. A verified `organization.created` with `org_id` and no `user_id` returns
2xx and reaches the organization handler.
2. A membership event without `user_id` is still rejected.
3. An organization event without `org_id` is rejected.
