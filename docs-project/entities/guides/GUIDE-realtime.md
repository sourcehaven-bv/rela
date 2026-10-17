---
id: GUIDE-realtime
type: guide
title: Realtime Channels (realtime.yaml)
status: published
order: 28
audience: advanced
summary: "Declare channel types, message types, roles, lifecycle and triggers in realtime.yaml, and how rela checks the file"
---

`realtime.yaml` declares the realtime channels of a project. A channel is a
conversation attached to an entity, such as an incident room on a data
breach. The file says which entity types get channels, who may read and
post, which kinds of message exist, and when rela posts a message by
itself.

The file is optional. Without it, realtime is off and nothing else changes.
`schema.yaml` never refers to it.

## Example

```yaml
version: 1
limits:
  post_rate: 20/m
channel_types:
  incident-room:
    label: Incident room
    attach_to: [datalek, incident]
    cardinality: one
    read: { from: attached }
    roles:
      lead:   { from: entity.owner, label: the incident lead }
      dpo:    { permission: privacy.officer }
      member: { from: attached }
      agent:  { principal_type: agent }
    message_types:
      text:
        properties: { body: { type: markdown, required: true, max: 8000 } }
        post: [lead, dpo, member, agent]
      status-update:
        properties:
          severity: { type: severity }
          summary:  { type: string, required: true }
        post: [lead]
        promote_to: { type: timeline-entry, map: { title: summary }, relate: { type: about } }
        actions:
          accept: { label: Accept, permission: incident.decide, promote_to: { type: decision } }
      typing: { persist: ephemeral, ttl: 5s, rate: 1/s, post: [lead, dpo, member, agent] }
    lifecycle:
      close:   { roles: [lead], auto: { when: "entity.status == 'closed'" } }
      reopen:  { roles: [lead, dpo] }
      archive: { roles: [lead], from: closed, auto: { after: 30d } }
      delete:  { permission: records.delete, from: archived, retention: 7y }
    message_ops:
      edit_own:   { roles: [lead, dpo, member, agent], within: 15m }
      delete_own: { roles: [lead, dpo, member], from: open }
      remove:     { roles: [lead], reason_required: true }
    on_entity_delete: block
triggers:
  breach-escalated:
    entity_type: datalek
    on: enter
    when: "entity.severity == 'high'"
    channel: incident-room
    post: { type: status-update, props: { summary: "Severity raised to high" } }
```

## Channel types

| Key | Meaning |
| --- | --- |
| `attach_to` | The entity types that get channels of this type. Name each type itself, not an alias. |
| `anchor` | `family` (default): one channel per entity id, across its faces. `face`: one channel per face. `face` needs every `attach_to` type to declare faces. |
| `cardinality` | `one` (default): one channel per entity, opened on first post. `many`: named channels that the `create` roles open. |
| `create` | Who may open a channel. Only for `cardinality: many`, and required there. |
| `read` | Who receives the channel. Default: `{ from: attached }`, everyone who can read the entity. Whatever the source, a reader must also be able to read the entity. |
| `roles` | Named groups of principals, used by every other key. |
| `message_types` | The kinds of message, at least one. |
| `lifecycle` | `close`, `reopen`, `archive` and `delete`. An operation you leave out is not offered. |
| `message_ops` | `edit_own`, `delete_own` and `remove`. |
| `on_entity_delete` | `detach` (default) keeps the channels, `delete` removes them, `block` refuses to delete an entity that has channels. |

`on_entity_delete` applies to deletes made through rela. An entity deleted
another way, such as a file removed by hand or by git, keeps its channels
as they were. Nobody can reach them: every channel request reads the entity
first, and a missing entity gives a 404. When rela creates a new entity
with the same id, it detaches those channels, so the new entity starts
without them. When the old entity comes back outside rela, for example
through a git checkout, it gets its channels back.

Names of channel types, message types, roles, actions and triggers use
lowercase letters, digits and hyphens, and start with a letter. Property
names use underscores instead of hyphens.

## Roles

A role has exactly one source. A role only ever admits principals; there is
no way to deny.

| Source | Matches |
| --- | --- |
| `permission: p` | A principal that holds `p` on the entity, through `acl.yaml`. |
| `from: attached` | A principal that can read the entity. |
| `from: entity.<property>` | The principal that a string property of the entity names. |
| `principal_type: t` | A principal whose verified principal type is `t`. |
| `creator: true` | The principal that opened the channel. Only for `cardinality: many`. |

Every channel read requires that the principal can read the entity. The
`read` role adds this to its source: `read: { permission: p }` admits a
holder of `p` only if that principal can also read the entity. A channel
never reveals itself to someone the ACL hides its entity from.

A client ceiling (`client_baselines` and `scope_grants` in `acl.yaml`)
narrows channel access in three ways:

- Reading a channel needs read access to the entity. The ceiling's read
  axis (`read`, `deny_read`, `worlds`, `deny_worlds`) narrows that.
- Every other operation also needs the ceiling to admit update of the
  entity's type (`update`, `deny_update`, `deny_write`), whichever role or
  permission admits the principal. These operations are opening a channel,
  posting, editing, deleting and removing messages, lifecycle operations
  and actions.
- A `permission:` role or gate is also narrowed by the ceiling's permission
  axis (`permissions`, `deny_permissions`). That axis does not cover
  writing: a client that keeps a permission still needs update access to
  write.

To keep an agent client read-only in channels, deny it writes:

```yaml
client_baselines:
  agents:
    applies_to: [service]
    deny_write: ["*"]
```

## Message types

| Key | Meaning |
| --- | --- |
| `persist` | `log` (default) stores messages. `ephemeral` keeps signals such as typing in memory for `ttl`, at most 60s. |
| `properties` | At most 32. Types: `string`, `markdown`, `integer`, `boolean`, `date`, `datetime`, `ref` (with `to:`), or an enum type from `schema.yaml`. `max` limits a string or markdown property in bytes. |
| `when` | A condition on the `message` record that a post must meet. |
| `post` | The roles that may post. Receiving needs only `read`. |
| `rate` | Posts per identity, such as `10/m`. It may only lower the limit. |
| `promote_to` | Turns a message into an entity: `map` copies message properties to entity properties, and `relate` links the new entity to the channel's entity. Only for `log` messages. A type that declares faces needs `face`, the face to create. |
| `actions` | Buttons on a message, each gated by `roles` or a `permission`. Only for `log` messages. |

## Lifecycle and periods

Each lifecycle operation names `roles` or a `permission`. `from` is the
state it starts from: `open`, `closed` or `archived`. `condition` must hold
for the entity. An operation with a `condition` is open only to principals
who can read the entity, because its answer says something about the
entity. `auto` runs `close` or `archive` without a request, when
`when` holds or `after` has passed. `retention` applies to `delete` only: a
channel cannot be deleted before it ends. Retention never blocks removing a
single message.

Periods are a count from 1 to 100000 and a unit: `s`, `m`, `h` and `d` have a fixed length;
`mo` and `y` are calendar months and years.

## Triggers

A trigger posts a message into an entity's own channel when the entity
starts matching `when` (`enter`), stops matching (`leave`), or changes a
property that `when` reads while it matches (`change`). The channel type
must have `cardinality: one` and attach to `entity_type`. `props` are
literal values: a trigger posts your text and a reference to the entity,
never a value read from it.

## Checks when the file loads

rela checks the whole file when it loads and lists every problem by key,
for example `realtime.yaml: channel_types.incident-room.attach_to[1]:
unknown entity type "nope"`. It checks:

- unknown keys, such as a misspelled one;
- entity types, relation types, enum types, properties and faces against
  `schema.yaml`;
- that some role in `acl.yaml` grants each `permission`;
- that every condition compiles;
- that each `promote_to` fills every required property of its type from a
  required message property, maps each property to one of a matching type,
  sets no computed property, and names a face when the type declares faces.

Three more rules stop a message from revealing what `acl.yaml` hides:

- A trigger's `when`, an `auto.when` and a lifecycle `condition` may read
  only `entity.<property>`.
  They may not follow relations, and may call only `contains`, `len`,
  `today`, `days_between` and `date_add`.
- They may not read a property that a `visible:` grant or a client
  baseline restricts. A message posted on a hidden value would reveal it to
  every channel reader.
- `from: entity.<property>` may not name such a property either.

## Limits

These are the built-in limits. The `limits:` block may only lower them. A
rate must be lower in both its average and its burst size, the count: `1000/h`
averages less than `30/m` but allows 1000 posts at once, so it is refused.

| Key | Default |
| --- | --- |
| `post_rate` | 30/m for `log` messages |
| `signal_rate` | 2/s for `ephemeral` messages |
| `message_bytes` | 16384 |
| `channels_per_entity` | 200 per channel type |
| `page_limit` | 200 |

## Reloading

- An invalid `realtime.yaml` stops the server from starting.
- rela-server reloads the file when it changes. If the new file is invalid,
  the previous one keeps working, and open browsers get a `config-error`
  event naming `realtime.yaml`.
- A change to `anchor:` of a channel type that has stored channels is
  refused, on reload and when the server starts, because the stored
  channels are keyed by the old anchor. Migrating channels to another
  anchor is not supported yet.
- Saving `schema.yaml` in the Configure space checks `realtime.yaml` against
  the new schema and refuses the save if it breaks. The Configure space
  cannot edit `realtime.yaml` itself.
- When the MCP server reloads a schema that breaks `realtime.yaml`, it keeps
  the previous realtime configuration in degraded mode and logs why. Triggers and
  automatic lifecycle rules do not run, operations with a `condition` are
  denied, and roles sourced from an entity property grant nobody.
- `rela acl audit` counts the permissions in `realtime.yaml` as in use,
  also when the file does not load. Only a file that is not valid YAML makes
  it skip the dead-permission check.
