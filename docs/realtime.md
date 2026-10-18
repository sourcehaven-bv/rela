<!-- This file is auto-generated from docs-project/entities/. Do not edit directly. -->

# Realtime Channels (realtime.yaml)

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
| `detached_retention` | How long a detached channel is kept before it is erased. A period of at least `1h`; default `30d`. |

### When an entity is deleted

`on_entity_delete` applies to deletes made through rela:

- `block` refuses the delete while the entity has channels of this type.
- `detach` moves the channels out of reach of the entity. They are erased
  after `detached_retention`.
- `delete` moves the channels out of reach in the same way, and erases
  them within a minute. The process that made the delete erases them at
  once. If another process is sweeping at that moment, the next check
  erases them; it runs every minute.

When an entity has channels of several types, each type follows its own
rule: the channels of a `delete` type are erased within a minute, and
those of a `detach` type are kept for their `detached_retention`.

A delete that fails puts the channels back. A channel of a `block` type
created while the delete runs still refuses the delete. A channel is never erased
before its `lifecycle.delete.retention` has passed, counted from when the
channel was closed. That is why rela refuses a file that combines
`on_entity_delete: delete` with `lifecycle.delete.retention`: use `detach`
with a `detached_retention` instead. A channel whose type the file no
longer declares is detached and kept for the default of 30 days.

A soft delete (moving the entity to the trash) keeps the channels in
place, so a restore finds them. They are moved out of reach when the
trash is emptied.

Erasing a channel removes its messages. Its audit trail is kept.

An entity deleted another way, such as a file removed by hand or by git,
keeps its channels for a while. Nobody can reach them: every channel
request reads the entity first, and a missing entity gives a 404. When the
entity has been missing for an hour, the sweep detaches its channels, and
`detached_retention` starts. If the entity comes back before they are
erased, for example through a git checkout, it gets its channels back.
When rela creates a new entity with the same id, it detaches the old
channels at once, so the new entity starts without them. From then on
the old channels can no longer come back, not even onto the new entity.

Renaming an entity to an id that an earlier, deleted entity used works
the same way: the earlier entity's channels are detached, and the renamed
entity keeps its own.

Only a long-running rela process sweeps: `rela-server` and the desktop
app. The sweep checks every minute for channels to close, archive or
erase, and checks for missing entities every hour. With several server
processes against one database, each check runs in one process only:
the first process to take a shared lock runs it, and the others skip
that check. If that process stops, another one runs the next check.

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
`when` holds or `after` has passed. `after` counts from when the channel
entered its current state; for a reopened channel, from the reopen.
`retention` applies to `delete` only: a
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
  sets no computed property, and names a face when the type declares faces;
- that no channel type combines `on_entity_delete: delete` with
  `lifecycle.delete.retention`.

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

## HTTP API

The data-entry API serves channels under the entity they are attached to.
`{plural}` and `{id}` are the entity's collection and id, as in
`/api/v1/{plural}/{id}`. On a type with faces, write the face as
`{id}@{face}`. A bare id on such a type gets a 422 `face_required`.

| Method and path | Does |
| --- | --- |
| `GET /api/v1/{plural}/{id}/_channels` | Lists the channels the caller may read, per channel type. `?state=` filters on a period. |
| `POST /api/v1/{plural}/{id}/_channels/{type}` | Opens a named channel of a `many` type. The body may hold a `title`. |
| `GET .../_channels/{type}` | The head of a `one` channel: its last sequence number, revision, and `_actions`, the operations the caller may perform. |
| `GET .../_channels/{type}/{name}` | The head of a named channel. |
| `GET .../{channel}/messages` | A page of messages. See below. |
| `POST .../{channel}/messages` | Posts a message: `{"type": ..., "props": {...}, "reply_to": n}`. A `log` message gets a 201 with the message; an `ephemeral` one gets a 202. |
| `PATCH .../{channel}/messages/{seq}` | Edits a message: `{"props": {...}}`. |
| `DELETE .../{channel}/messages/{seq}` | Removes a message. The body may hold a `reason`. |

`{channel}` is `_channels/{type}` or `_channels/{type}/{name}`.

A message page takes one cursor:

- `?after_rev=r` returns the messages that changed after revision `r`, so a
  client catches up after a live frame. The answer has `next_after_rev`.
- `?after=n` returns the messages after sequence number `n`, oldest first.
  The answer has `next_after`.
- `?before=n` returns the messages before `n`, for scrolling back. The
  answer has `next_before`.
- Without a cursor, the page holds the newest messages. The answer has
  `next_before`, to scroll back from there.
- `?limit=` caps the page. The default and the maximum come from
  `page_limit`.

Each page names one cursor, the one that continues it in the direction it
was read. An empty page names none. `has_more` means the page is full: there
may be more messages past the cursor, and only the next request can tell.

A `ref` property is served with the target's id, type and display title. A
target the caller cannot read, or that no longer exists, is served as
`{"restricted": true}`, without its id.

### Errors

Every error is a problem body (`application/problem+json`) with a `type`.

| Status | When |
| --- | --- |
| 404 `not_found` | Realtime is off, or the entity is hidden, does not exist, or is not of the collection's type. The body is the same as for the entity `GET`. |
| 404 | The channel type is not attached to the entity type, or the channel or the message does not exist. |
| 403 | The caller may not perform the operation, or the edit window has passed. `title` says what the operation needs and `detail` names the rule in `realtime.yaml`, such as `channel_types.room.message_types.text.post`. |
| 400 | The message does not match its type (`invalid_message`), has too many properties, or a removal lacks a required reason. |
| 400 | The body is not valid JSON, has unknown fields, or a query parameter is not a positive number. |
| 413 `too_large` | The message is over `message_bytes`. |
| 422 | A `ref` names an entity that does not exist or that the caller cannot read. Both get the same body. |
| 409 | The channel is not open, the message was removed, `channels_per_entity` is reached, the POST tried to open a channel of a `one` type, or the configuration changed during the request. |
| 429 | A rate limit. `Retry-After` gives the seconds to wait, rounded up. |

The OpenAPI document at `/api/v1/_openapi.json` lists these paths for every
entity type a channel type attaches to, with one request body per message
type. It follows a reload of `realtime.yaml`.

### Live updates

Two channel paths can be subscribed to on the live event stream: the
channel list (`.../_channels`) and a channel head (`.../_channels/{type}` or
`.../_channels/{type}/{name}`). A message path cannot. A frame on a head
means the channel changed: fetch `messages?after_rev=` with the revision you
last saw. A frame on the list means a channel of the entity was opened,
closed, or got a message.

An `ephemeral` message changes the head only. It is not stored, so a client
that was not subscribed when it was posted does not see it.

The server re-checks a subscription only when its channel changes. A
subscription to a channel the caller cannot read gets no frames. A reader
who loses access gets one frame, and no frames after that. On that frame the
head answers 404 when the entity is no longer readable, or 403
`read_denied` when the entity stays readable but the channel's `read:` rule
no longer admits the reader. A reload of `realtime.yaml` re-checks every
subscription at once, so a reader it revokes gets that frame at the reload,
not at the channel's next message.

A single server process delivers frames for the messages posted through it;
it does not yet poll for messages posted by another process.

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
