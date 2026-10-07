---
id: FEAT-XYQMUB
type: feature
title: Integration sync with external systems via Lua connectors
summary: 'Infrastructure for 3-way sync between rela entities and external systems: external refs, ACL-enforced field ownership, history-based merge and Lua connectors'
description: Infrastructure so an entity can be synced with its counterpart in an external system. The link is an external-ref property, ownership is enforced by ACL field grants, the 3-way merge uses store history, and each integration is a Lua connector run by automations, the scheduler and webhooks. Alternative to the twins proposal (mellelieuwes/rela#1).
priority: medium
status: proposed
---

## Problem

An entity in rela often has a counterpart in another system, such as a Basecamp
todo or a GitHub issue. rela has no shared way to say "this entity has a
counterpart, and that system owns these fields". Each integration models the
other side on its own terms (CalDAV `read_only:`, the sync hash index, webhooks
without memory).

## Direction

This is the alternative to the twins proposal (mellelieuwes/rela#1). That PR
adds a parallel subsystem: its own twin store in `.rela/`, a field-ownership
guard in `entitymanager`, and a special sync write handle. Here, rela provides
general infrastructure, and each integration is a Lua script on top of it.

- **Link**: an external-ref property in the graph. It travels with the
project, works on every database backend, and gets ACL, audit and history.
- **Ownership**: ACL field write grants, held by the integration's system
principal. Requires enforcing `FieldWriteGate` on every write path (TKT-0XL8MF).
- **3-way sync**: the base is the entity version at the last sync, read
through a Lua history API.
- **Transport**: Lua connector scripts. Pushes run as background jobs
enqueued by an automation; pulls run on the scheduler or from a webhook.
- **Credentials**: the secret store, plus a writable binding for rotating
OAuth refresh tokens. A reference script does the one-time OAuth consent step
outside rela.

## Decisions

- **Loop prevention is by convergence against the base, not by principal.** A
push sends only fields that differ from the base, and every pull or push moves
the base. Echoes then stop by themselves.
- **Bare fs is not supported for sync.** It has no store-level history.
sqlite covers small deployments. A backend without versioning refuses
sync-managed refs at load.
- **The sync writes as a system principal** (for example `system:basecamp`),
never as the user who connected the external account.

## Tickets

1. fs-to-sqlite migration command, and a sqlite build for desktop.
2. Lua history API.
3. Enforce ACL field write grants on every write path (TKT-0XL8MF).
4. Automation action that enqueues a Lua script as a background job.
5. External-ref property type and a Lua 3-way merge helper.
6. Writable token binding, reference OAuth script and a Basecamp reference
connector.
