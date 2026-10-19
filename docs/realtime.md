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
        actions:
          log:
            label: Log
            roles: [lead]
            promote_to: { type: timeline-entry, map: { title: summary }, relate: { type: about } }
          accept:
            label: Accept
            permission: incident.decide
            promote_to: { type: decision, map: { title: summary } }
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
processes against one database, one process leads and runs every
check: it holds a shared lock while it runs. If that process stops, or
the database drops its connection, another one takes over within ten
seconds. The lock keeps one database connection for as long as it is
held, so with postgres `pool_max_conns` must be at least 3.

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
| `actions` | Buttons on a message, each gated by `roles` or a `permission`. An action may have a `promote_to`. Only for `log` messages. See [Actions and promote](#actions-and-promote). |

## Actions and promote

An action is a button on a message, such as `ack` or `accept`. A message
carries at most one action. Once someone runs an action on a message, no
other action can run on it. Actions run only while the channel is open.

`promote_to` on an action turns the message into an entity:

- `type` is the entity type to create. It must have generated ids; a type
  with `id_type: manual` is refused when the file loads.
- `map` copies message properties to entity properties, as
  `entity_property: message_property`.
- `relate` links the new entity to the channel's entity with a relation
  type, from the new entity to the channel's entity.
- `face` is the face to create, and is required when the type declares
  faces.

A message type cannot have a `promote_to` of its own. Put it on an action,
so the action's gate decides who may promote.

The entity is created as the person who runs the action. The create ACL,
the `fields:` grants in `acl.yaml`, validation and automations apply, as
they do to a create through the entity API. The audit records of the
create carry a `triggered_by` such as
`channel:INC-7/incident-room#12/log`: the channel, the message sequence
number and the action.

A `ref` property that `map` copies is checked as the person who runs the
action, not as the poster. If that person cannot read the entity, or it no
longer exists, the promote is refused with `promote_invalid`. The answer
names the message property and not the entity.

While the promote runs, the message shows the action as `pending`. A
pending message cannot be edited, removed or take another action. When the
entity exists, the action becomes `done` and names the entity.

What happens when a promote fails depends on the storage backend:

- **PostgreSQL and SQLite.** The entity, its relation and the action
  commit in one transaction. If any of them fails, none is written, and the
  message is free for another action. A failing automation after the
  commit does not undo the entity; the answer carries a
  `promote_followup_failed` warning instead.
- **Files (the default).** The entity is written first, then the action.
  If the create is refused before anything is written, such as by the ACL
  or validation, the message is free again. If the relation fails, the
  entity stays and the answer carries a `promote_relate_failed` warning.
  If an automation after the create fails, the entity stays and the
  answer carries a `promote_followup_failed` warning. If marking the
  action fails after the entity exists, the answer carries a
  `promote_unconfirmed` warning. For any other failure the action stays
  `pending`, because rela cannot tell whether the entity was written.

When a promote fails and the message is free again, the channel audit log
records the action with the status `reverted`.

A pending action that stays pending can be released, so the message can
take an action again. Check first whether the entity exists. Releasing
needs `message_ops.remove`, or the action's own gate when the channel type
declares no `remove`. The action must have been pending for 15 minutes,
so a promote that is still running is not released under it. This wait is
the release window. The channel audit log records each release with the
status `released`.

A message lists the actions the caller may run on it now in `_actions`.
It runs the same checks as the `POST`, except the rate limit and the
`ref` check above. A promoting action is offered only to a caller the
create ACL lets create its entity. A process that cannot write the graph
does not offer a promoting action and answers its `POST` with 403
`action_not_offered`.

## Lifecycle and periods

Each lifecycle operation names `roles` or a `permission`. `from` is the
state it starts from: `open`, `closed` or `archived`. `condition` must hold
for the entity when a person asks; an `auto` step does not check it. An operation with a `condition` is open only to principals
who can read the entity, because its answer says something about the
entity. `auto` runs `close` or `archive` without a request, when
`when` holds or `after` has passed. `after` counts from when the channel
entered its current state; for a reopened channel, from the reopen.
`retention` applies to `delete` only: a channel cannot be deleted until
`retention` has passed since it closed. A channel that never closed must be
closed first. Retention never blocks removing a single message.

Without `from`, `close` starts from `open`, `reopen` from `closed` or
`archived`, `archive` from `open` or `closed`, and `delete` from any state.

`auto.when` acts once, when the entity starts to meet the condition. It
moves the entity's channels of the type that are in a `from` state, and
leaves the others alone. A person can reopen such a channel while the
condition still holds, and the reopen stands: rela acts again only when
the entity stops meeting the condition and then meets it again. A new
`auto.when`, or one whose condition you change, starts from now: rela
first records which entities meet it, without moving their channels. It
creates no channel either: an entity whose channel was never written has
nothing to close.

`auto.when` and `auto.after` ignore the operation's `roles`,
`permission` and `condition`: the rule is its own condition. So whoever
may change the properties an `auto.when` reads can close or archive the
entity's channels, and can close a reopened channel again by moving the
entity out of the condition and back. Treat write access to those
properties as part of the operation's control, not only its `roles`.

An automatic close or archive has the author `system:realtime`, and its
audit record names the rule, for example `triggered_by:
auto:incident-room/close`. The person whose change set it off is in the
entity's history, not in the channel's. `auto.after` and `auto.when` of one operation
share that label. Both use the compare-and-set a person's request uses,
so a channel a person moved first is left as it is. `auto.when` runs in
the same worker as triggers and follows the same rules (see Triggers):
it looks at changed entities twice a second, catches missed changes every
15 minutes, acts at most once a minute per entity, and stops while the
file is not valid. Each channel it moves counts against the 64 attempts
per look; the rest move at the next look. A trigger into the same channel
type that posts about the same entity goes first, so its message lands
before the channel closes. When that post waits for its once-a-minute
limit or for a retry, the close waits too, up to a minute.
On a type with faces, the same face rule as for triggers applies.

Deleting a channel removes it and all its messages. A `one` channel starts
again, empty, at the next post.

Periods are a count from 1 to 100000 and a unit: `s`, `m`, `h` and `d` have a fixed length;
`mo` and `y` are calendar months and years.

## Triggers

A trigger posts a message into an entity's own channel when the entity
starts matching `when` (`enter`), stops matching (`leave`), or changes a
property that `when` reads while it matches (`change`). The channel type
must have `cardinality: one` and attach to `entity_type`. `props` are
literal values: a trigger posts your text and a reference to the entity,
never a value read from it.

| Key | Meaning |
| --- | --- |
| `entity_type` | The type whose entities the trigger watches. |
| `on` | `enter`, `leave` or `change`. |
| `when` | The condition, over `entity.<property>`. |
| `channel` | The channel type to post into. |
| `post` | The message `type` and its literal `props`. |
| `when_closed` | `skip` (default) posts nothing while the channel is closed or archived; `post` posts anyway. |

The message type must be a `log` type: a signal would reach only the
readers connected to the process that runs triggers. rela checks the
`props` against the message type when the file loads, including each
property's `max`, `message_bytes` and the type's `when`. A message type
whose `when` calls `today()` cannot be a trigger's: whether the `props`
meet it would change with the date.

A trigger's message has the author `system:realtime`. The API marks it
with `author.system: true` and `triggered_by: trigger:<name>`, so a client
can show it apart from a person's message. A trigger needs no role in
`acl.yaml` and is not held to `post_rate`.

How triggers run:

- **They start from now.** A new trigger, or one whose `entity_type` or
  `when` you change, first records which entities match, without
  posting. Only changes after that post.
- **Changes close together are seen together.** rela looks at changed
  entities twice a second. An entity that starts and stops matching in
  between posts nothing. One trigger posts at most once a minute about one
  entity; the changes in between are seen together when the minute ends.
- **Posts are capped.** rela tries at most 64 trigger posts per look,
  shared by the triggers in turn, so one busy trigger cannot hold back
  the others. The rest wait their turn, first come, first served.
- **Deleting is not leaving.** A deleted entity posts no `leave`. When it
  comes back, for example through a restore, it posts `enter` again. A
  renamed entity counts as a new one and posts `enter` under its new id.
- **A missed change is caught later.** Every 15 minutes, and when the
  process starts, rela compares every entity with what it has seen. That
  catches a change that raised no event, and a `when` on `today()` that
  starts to hold as the date changes. `today()` is the date in UTC.
- **A post is delivered at least once.** A crash between posting and
  recording the post can post it again. A post that fails, for example
  while the database fails over, is retried with a growing wait for five
  minutes, then logged and counted as done, so it never repeats forever.
  A post that cannot succeed, such as one the channel's limits refuse,
  is logged and counted as done at once.
- **Only a long-running process runs triggers:** `rela-server` and the
  desktop app. With several server processes, one leads: it holds the
  realtime worker lock while it runs, and runs the triggers and the
  sweep. The others try to take over every ten seconds, and the one that
  does compares every entity first. The CLI and the MCP server run none:
  their changes reach a running server and post there.
- **Without a valid file, triggers stop.** While `realtime.yaml` is
  missing or kept from an earlier load after an error, no trigger posts.
  When the file loads again, the next comparison catches up.

On a type with faces, a trigger on a channel type with `anchor: face`
watches each face on its own and posts into that face's channel. A
trigger on a channel type anchored to the whole entity reads the default
world's face. rela leaves such a trigger out, with a warning in the log,
when `acl.yaml` lets some readers see only certain faces: its message
would describe a face they may not see. The rest of the file stays in
force.

A trigger creates the channel when it posts first. Under
`on_entity_delete: block`, that channel then blocks deleting the entity,
as a channel a person started does.

## Checks when the file loads

rela checks the whole file when it loads and lists every problem by key,
for example `realtime.yaml: channel_types.incident-room.attach_to[1]:
unknown entity type "nope"`. It checks:

- unknown keys, such as a misspelled one;
- entity types, relation types, enum types, properties and faces against
  `schema.yaml`;
- that some role in `acl.yaml` grants each `permission`;
- that every condition compiles;
- that `promote_to` is on an action, not on a message type, and names a
  type with generated ids;
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

rela refuses the whole file for any of these problems, except one: a
trigger or an `auto.when` it must leave out because of faces (see
Triggers) is dropped with a warning, and the rest of the file stays in
force.

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
| `POST .../{channel}/messages/{seq}/_action/{name}` | Runs an action. The body is empty or `{}`. The answer is `{"message": ..., "warnings": [...]}`. |
| `DELETE .../{channel}/messages/{seq}/_action` | Releases a pending action. The body is empty or `{}`. The answer is the message. |
| `POST .../{channel}/_lifecycle/{op}` | Runs `close`, `reopen`, `archive` or `delete`. The body is empty or `{}`. A `delete` gets a 204; the other operations get the channel. |

`{channel}` is `_channels/{type}` or `_channels/{type}/{name}`.

The head's `_actions.lifecycle` lists the lifecycle operations the caller
may run now. It runs the same checks as the `POST`, except the rate limit.
A `one` channel that holds no message has no lifecycle operations.

A message page takes one cursor:

- `?after_rev=r` returns the messages that changed after revision `r`, so a
  client catches up after a live frame. `r` may be 0. The answer always has
  `next_after_rev`, also when the page is empty.
- `?after=n` returns the messages after sequence number `n`, oldest first.
  The answer has `next_after`: the last sequence number on the page, or `n`
  when the page is empty.
- `?before=n` returns the messages before `n`, for scrolling back. The
  answer has `next_before` when the page holds a message.
- Without a cursor, the page holds the newest messages. The answer has
  `next_before`, to scroll back from there, and `next_after_rev`, the
  channel's revision, to follow the channel from there.
- `?limit=` caps the page. The default and the maximum come from
  `page_limit`.

A page names the cursor that continues it in the direction it was read. The
newest page also names `next_after_rev`. A message posted between reading the
channel and reading the page can then show up twice, but never goes missing,
so a client removes duplicates by `seq`. `has_more` means the page is full:
there may be more messages past the cursor, and only the next request can
tell.

The cursors are the same on the HTTP API, the MCP tools and the CLI. Before
this rule, an empty `after_rev` page named no `next_after_rev`.

A `ref` property is served with the target's id, type and display title. A
target the caller cannot read, or that no longer exists, is served as
`{"restricted": true}`, without its id.

### Errors

Every error is a problem body (`application/problem+json`) with a `type`.

| Status | When |
| --- | --- |
| 404 `not_found` | Realtime is off, or the entity is hidden, does not exist, or is not of the collection's type. The body is the same as for the entity `GET`. |
| 404 | The channel type is not attached to the entity type, or the channel or the message does not exist. |
| 403 | The caller may not perform the operation, the edit window has passed, the lifecycle operation is not declared (`lifecycle_not_offered`), or its `condition` does not hold (`lifecycle_denied`). For actions: the message type does not declare the action, or this process cannot promote (`action_not_offered`), the action's gate refuses (`action_denied`), or the create ACL or a `fields:` grant refuses the promote (`promote_denied`). `title` says what the operation needs and `detail` names the rule in `realtime.yaml`, such as `channel_types.room.message_types.text.post`. |
| 400 | The message does not match its type (`invalid_message`), has too many properties, or a removal lacks a required reason. |
| 400 | The body is not valid JSON, has unknown fields, or a query parameter is not a positive number. |
| 413 `too_large` | The message is over `message_bytes`. |
| 422 | A `ref` names an entity that does not exist or that the caller cannot read. Both get the same body. |
| 422 `promote_invalid` | The promoted entity or its relation fails validation, for example a `unique` property that another entity already has, or a mapped `ref` names an entity the caller cannot read. The body names properties, never entity ids. |
| 409 | The channel is not open, the channel is not in a state the lifecycle operation starts from (`lifecycle_wrong_state`), `retention` keeps the channel (`retention_active`), a concurrent change won (`conflict`), the message was removed, `channels_per_entity` is reached, the POST tried to open a channel of a `one` type, or the configuration changed during the request. For actions: the message already carries an action (`action_taken`), its action is pending (`action_pending`, also for an edit, a removal, a release within the release window, or an action that lost a race to a promote that started first), a release finds no pending action (`action_not_pending`), or the promoted entity already exists (`promote_conflict`). |
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

## Agents and other processes

Agents use channels through the MCP channel tools (see the MCP server
guide). Remote MCP runs inside rela-server; stdio `rela mcp` is a separate
process.

**Process locality.** Signals (`ephemeral` messages), rate-limit buckets and
change subscriptions live in the memory of one process. A separate process,
such as stdio `rela mcp`, therefore behaves differently from rela-server:

- It refuses a post of an `ephemeral` message type with `signals_not_kept`,
  because no one else would see the signal. Remote MCP keeps signals.
- Its rate limits count per process, not across all writers.
- A `watch` on `read_channel` learns about posts from other processes by
  polling every 2 seconds. On remote MCP the poll runs every 5 seconds and
  only matters for posts made by another rela-server process; a post in the
  same process ends the wait at once.
- A post from another process reaches open browsers only when they fetch the
  channel again. rela-server does not send a live frame for it.

**SQLite.** The SQLite backend admits one process at a time. While
rela-server runs on it, stdio `rela mcp` cannot open the project, and remote
MCP is the only way for an agent to use channels.

**Access control.** Stdio `rela mcp` leaves its entity tools open, because
the local user can edit the files anyway. Its channel tools apply `acl.yaml`
when the project has one, as `rela lease` does, so an agent identity stays
inside the channel roles the project gives it. Locally this is not a
security boundary: a local user can edit `acl.yaml`. Without `acl.yaml`,
access is open.

The channel tools act as `$USER`. When `acl.yaml` exists and its
`assignments` give that user no role, and no user entity or membership
gives it one, the channel tools can answer `entity not found` for an entity that
`show_entity` returns, and `rela mcp` logs a warning at startup. Add the
user under `assignments`. `rela mcp` reloads `acl.yaml` when it changes.
An `acl.yaml` that does not apply makes every channel tool answer
`acl.yaml cannot be loaded: <reason>` until it is fixed.

**Prompt injection.** Message content is written by other people and agents.
Every channel tool answer that carries such content starts with a `notice`
that says the content is untrusted user data, and the content is delivered as JSON values. A model
may still follow instructions written in a message. That risk remains. Limit
what an agent may write: give its identity only the roles it needs, and keep
it out of roles that post on channels other agents act on.

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
