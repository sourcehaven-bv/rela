---
id: TKT-55RNWWD
type: ticket
title: 'Direct messages on channels: decide the DM model'
kind: enhancement
priority: medium
effort: m
started: "2026-11-10"
completed: "2026-11-10"
status: done
description: 'Design discussion: how direct messages between principals fit on realtime channels, which today always hang off an entity.'
---

## Description

Discuss and decide how direct messages (DMs) between principals fit on top of
realtime channels. Today every channel hangs off an entity (`channels.Anchor` is
an entity id plus face), and every channel read requires read access to that
entity. A DM has no natural entity: it is a conversation between people (and
possibly agents).

This ticket is a design discussion first. Implementation tickets follow once the
model is decided.

## Open questions

- Anchor: a DM anchored on an entity (a `conversation` entity in the graph, or
the user entity) or a new principal-set anchor outside the graph?
- Participants: 1:1 only, or group DMs? Can membership change?
- Agents: may a person DM an agent (job identity), and an agent a person?
- Reachability: who may DM whom? Anyone who can read the target's user
entity, an ACL permission, or a realtime.yaml role?
- Config: built-in DM channel type, or declared in `realtime.yaml` like other
channel types (message types, actions, promote_to)?
- Retention, audit and operator access (compliance projects such as an ISMS).
- Inbox, unread counts, mentions and the SPA entry point for DMs.

## Current model (constraints found)

- `channels.Key` is `Anchor{ID, Face}` + type + name. The anchor is always an
entity id (`internal/channels/channels.go`).
- Every channel read requires read of the anchor entity; every other operation
requires update of the anchor's type under the client ceiling
(`realtimecfg.Source.NeedsCeilingCheck`).
- Role sources: `permission`, `from: attached`, `from: entity.<property>`
(one principal), `principal_type`, `creator`. None names a set of people.
- Entity delete, the sweep, triggers, promote `relate`, live topics and inbox
links all assume an anchor entity.
- Comments are deliberately kept out of the graph (no entity, audit or
versioning); the same question applies to private messages.

## Options

### A. Conversation entity in the graph

The operator declares a `conversation` entity type with participants and a
`cardinality: one` channel type on it. Gap: a role source that names a set of
principals (list property or `related(...)`).

- For: almost all machinery is reused (ACL, inbox, lifecycle, promote).
- Against: every DM becomes a graph entity: audited, versioned, searchable and
readable by anyone the ACL lets read the type. Private chatter enters the domain
graph. Privacy depends on per-project ACL authoring.

### B. Principal-set anchor outside the graph

A new anchor kind: the sorted set of participant principals. Access is
membership (plus the client ceiling), not entity read. Message types are
declared in `realtime.yaml` (a `direct:` block), so actions and `promote_to`
still work, with `relate` optional. Routes under `/api/v1/_direct/...`.

- For: the right privacy model; no graph pollution; matches the comments rule.
- Against: the anchor-is-an-entity assumption runs through the service,
sweep, triggers, live hub and inbox. A new authorization basis needs a security
review. Effort XL, split into several tickets.

### C. Anchor on the user entity

`attach_to: [user]`. Rejected: asymmetric (whose user entity?) and cannot
express the other participant.

## Decisions (2026-11-10)

- Scope: 1:1 only, people only (no agents).
- No operator read access yet, but the design must leave room for an
`acl.yaml` permission later.
- Model: option A, but as a rela-managed type in the style of `_flow-run`
(`internal/metamodel/flowrun.go`, `internal/acl/managed.go`), not an
operator-declared one.

## Proposed design: managed `_dm` entity

**Type.** rela adds a managed type `_dm` (id prefix `DM-`) when `realtime.yaml`
declares a `direct:` block. `schema.yaml` cannot declare it; `acl.yaml` cannot
name it; `"*"` does not cover it.

**Participants.** Two managed `_member` edges from the `_dm` to the two user
entities. Person mapping (`user_entity_type` + `principal_property`) is
required; `realtime.yaml` with `direct:` fails to load without it.

**Id.** Derived from the sorted pair of user entity ids (a hash), so "open the
DM with X" is idempotent and two concurrent opens cannot create two
conversations.

**Who can read.** Exactly the two participants: a compiled allowlist route "the
principal's user entity is the target of a `_member` edge", lowered to a store
query like `related(...)`. Everyone else gets the uniform 404, so existence of a
DM stays secret. Later, operator access is one more grant on this route set (an
`acl.yaml` permission), not a change of model.

**Who can write.** Only a managed role for `system:realtime`, as with
`system:flow`. Opening a DM is a service call: the caller must be a person, the
target must be a person whose user entity the caller can read, and the caller
cannot DM themself.

**Messages.** The channel machinery is reused unchanged: a managed channel type
`_direct`, `attach_to: [_dm]`, `cardinality: one`, with a managed role source
"participant". The `direct:` block declares message types, `typing`, and
`message_ops` with the existing grammar. No lifecycle, sessions, triggers or
`promote_to` in v1.

**Graph hygiene.** No automations, validations, comments, import, rename or
convert on `_dm` (same rules as `_flow-run`). Excluded from search, menus and
list pages; it shows only through the inbox and a DM entry point.

**Deletion.** Deleting a user entity removes its `_member` edge. The DM and its
channel are then detached and erased after `detached_retention`.

## Open points

- Does the `_dm` header need search exclusion beyond the row gate, or is
the gate enough?
- Where the SPA shows DMs: an inbox section, a "Message" button on the user
entity page, or both.
- Whether `direct:` lives in `realtime.yaml` (recommended) or is built in.

## Follow-up tickets (after approval)

1. Managed `_dm` type, `_member` edges, ACL route and `system:realtime` role.
2. `direct:` config block and managed `_direct` channel type with the
participant role source.
3. Open-DM service call, HTTP route, MCP tool and CLI.
4. SPA: DM entry point and inbox section.
