---
id: TKT-IZL6NX8
type: ticket
title: Store IdP-asserted roles per principal for background jobs (R14)
kind: enhancement
priority: medium
effort: m
started: "2026-11-06"
completed: "2026-11-06"
status: done
---

## Description

`asserted_role_assignments` maps the `roles` claim of the current verified JWT
to ACL roles. Outside a request there is no JWT. A scheduler `for_each` child
runs as its selected user entity with graph roles only, and a Lua script cannot
ask which users hold a role. OpenVWR needs both to mail the privacy officer and
to send a mandate-holder digest (openvwr2 `docs/04-platform-gaps.md`, R14).

## Approach

- The IdP webhook action (`POST /webhooks/idp`) may return
`asserted_roles = {...}`. The receiver stores those IdP role names for the
subject named by the verified webhook (`user_id`), with its `org_id`. An empty
list removes the record. Only the webhook path honours the key; the HTTP action
endpoint refuses it.
- Storage: new package `internal/idproles`, one document in `state.KV` written
under a `lock.Locker`. On postgres both are database-backed (tenant schema
`state_kv` and a schema-scoped advisory lock), so several rela-server processes
share one record set. Not a graph property: a property is writable through the
entity write path, so a user with update rights on a person could grant roles.
- A scheduler `for_each` child whose user entity has stored roles runs with a
principal carrying those roles as its verified roles claim. The roles still pass
through `asserted_role_assignments`. The principal_property value must map back
to exactly that entity.
- New Lua read `rela.principals_with_role(role)`: the user entities whose
stored IdP roles map to `role`, filtered by the script's gated reader.

## Out of scope

- Stored roles on other non-JWT paths (CLI, MCP stdio, header principal).
- Org enforcement (R1).
- Expiry of stored roles: TKT-TZSC2R0.
- Checking the asserted org before storing: TKT-YWX30D6.
