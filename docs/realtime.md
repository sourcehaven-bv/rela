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
    typing: true
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
    lifecycle:
      close:   { roles: [lead], auto: { when: "entity.status == 'closed'" } }
      reopen:  { roles: [lead, dpo] }
      archive: { roles: [lead], from: closed, label: Archive the room, auto: { after: 30d } }
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
| `typing` | `true` turns on the built-in typing indicator. Default `false`. See Typing indicator. |
| `lifecycle` | `close`, `reopen`, `archive` and `delete`. An operation you leave out is not offered. Each may have a `label` for its button; without one, the label is the operation's name. |
| `message_ops` | `edit_own`, `delete_own` and `remove`. |
| `on_entity_delete` | `detach` (default) keeps the channels, `delete` removes them, `block` refuses to delete an entity that has channels. |
| `detached_retention` | How long a detached channel is kept before it is erased. A period of at least `1h`; default `30d`. |

### Typing indicator

With `typing: true`, a reader sees who else is typing a message in the
channel, such as "Ann is typing…". It needs no message type and no other
setup.

- **Who may send it.** Anyone who may post at least one `log` message type
  of the channel, while the channel is open. The head's `_actions.typing`
  says whether the caller may send it now.
- **Who sees it.** Every reader of the channel, except the typist. Readers
  see the author and since when they type, in `typing_now`. A closed or
  archived channel shows nobody.
- **How long it lasts.** An entry lives for 6 seconds. A client refreshes it
  at half of that while the user types. A `log` post by the typist ends the
  entry at once; otherwise the server ends it when the 6 seconds pass.
- **When readers hear of it.** Readers get a live frame when someone starts
  typing, when a post ends the entry, and when the server ends it. A refresh
  sends no frame.
- **Limits.** One typing call per second per person and channel. Typing
  calls also count toward the 300 channel writes per minute that one person
  may make across all channels, posts included. At most 32 people type in
  one channel and 4096 channels hold typing entries in one process. Past a
  cap the call gets a 429; no entry is pushed out, and no rate is charged.
  `signal_rate` does not apply.
- **What it does not do.** An entry is never stored and never audited, and
  typing creates no channel. Entries live in the memory of one process,
  apart from `ephemeral` signals, so the two never take each other's place.

A channel type with `typing: true` cannot also declare a message type
named `typing`. Without the key, a message type named `typing` works as
before.

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
| `label` | The name the web app shows for the type. Without it, the type's name. |
| `persist` | `log` (default) stores messages. `ephemeral` keeps signals, such as a status line, in memory for `ttl`, at most 60s. For a typing indicator, use `typing: true` on the channel type. |
| `properties` | At most 32. Types: `string`, `markdown`, `integer`, `boolean`, `date`, `datetime`, `ref` (with `to:`), or an enum type from `schema.yaml`. `max` limits a string or markdown property in characters (Unicode code points). A property may have a `label`; without one, the label is its name. A `ref` property may set `notify: false`, so a value that names a person does not mention them (see [Mentions and replies](#mentions-and-replies)). `notify` on any other type is a config error. |
| `when` | A condition on the `message` record that a post must meet. |
| `post` | The roles that may post. Receiving needs only `read`. |
| `in` | The session states in which the type may be posted. Without it, every state. Only on a channel type with a `session`. See [Sessions](#sessions). |
| `rate` | Posts per identity, such as `10/m`. It may only lower the limit. |
| `actions` | Buttons on a message, each gated by `roles` or a `permission`. An action may have a `promote_to`. Only for `log` messages. The names `edit`, `delete`, `remove` and `release` are reserved. See [Actions and promote](#actions-and-promote). |

Labels are authored, never derived: rela shows a name as written when no
`label` is given.

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
`ref` check above. Next to the action names, `_actions` lists the message
operations the caller may use now:

- `edit`: the author may edit the message. It disappears when the edit
  window passes, when the channel is not open, and while an action is
  pending. A `ref` property the caller cannot read does not stop an edit:
  see the `PATCH` row below.
- `delete`: the author may delete their own message (`delete_own`).
- `remove`: the caller may remove the message (`remove`), with a reason
  when the channel type requires one.
- `release`: the caller may release the message's pending action now.

That is why an action cannot take one of these names.

Some of these change with time, without a new revision of the message.
So a message names when they will change:

- `edit_until`: when `edit` disappears, because the edit window ends.
  Only present while the message offers `edit` and `edit_own` has a
  `within`.
- `release_after`: when `release` appears for a caller the release gate
  admits, because the release window ends. Only present while the action
  is pending and the window has not passed.

A client sets a timer for the soonest of these and reads the message
again when it passes. A change of `acl.yaml` or of a role can also change
`_actions` without a new revision; the live frame it causes carries no
message, and the SPA then reads the newest page again. An operation the
caller may no longer perform is refused either way. A promoting action is offered only to a caller the
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

## Sessions

A session tracks where a conversation is, such as the steps of a breach
response. It is a set of states declared on a channel type. Typed messages
move the channel from one state to the next. The lifecycle (`open`,
`closed`, `archived`) says whether people can post; the session says what
the conversation is doing.

```yaml
channel_types:
  breach-response:
    label: Breach response
    attach_to: [datalek]
    roles:
      lead:   { label: the incident lead, from: entity.owner }
      member: { from: attached }
      dpo:    { label: the privacy officer, permission: privacy.officer }
    message_types:
      text:      { properties: { body: { type: markdown, required: true } }, post: [member] }
      contained: { properties: { note: { type: markdown } }, post: [lead, member] }
      assessed:  { properties: { risk: { type: risk-level, required: true } }, post: [lead, dpo] }
      notified:  { properties: { reference: { type: string, required: true } }, post: [dpo] }
      dismiss:   { properties: { reason: { type: string, required: true } }, post: [lead] }
      escalated: { properties: { note: { type: string } }, post: [lead] }
      reminder:         { properties: { body: { type: markdown, required: true } }, post: [lead] }
      escalate-stalled: { properties: { note: { type: string } }, post: [dpo] }
      handoff:          { properties: { to: { type: ref, to: [risico], required: true } }, post: [lead] }
      accept-handoff:   { properties: { note: { type: string } }, post: [lead] }
      claim:            { properties: { note: { type: string } }, post: [member] }
    session:
      label: GDPR breach response
      initial: detected
      on_final: close
      run_as: system:breach-bot
      states:
        detected:
          label: Detected
          on:
            contained:
              to: contained
              roles: [lead]
              outputs:
                - create:
                    type: timeline-entry
                    properties:
                      title: "Contained by {{message.author}}: {{message.note}}"
                      at: "{{now}}"
                    relate: { type: about }
            dismiss:   { to: dismissed }
            escalated: { to: contained, by: trigger }
          after:
            no-containment: { period: 24h, to: overdue, label: Not contained within a day }
        contained:
          label: Contained
          waiting_for: [lead]
          on:
            assessed:
              to: assessed
              roles: [lead]
              when: "message.risk ~= 'none'"
              outputs:
                - set: { properties: { risk: "{{message.risk}}" } }
            escalate-stalled: { to: overdue, when: "session.reminders['status-update'] >= 3" }
            handoff:
              to: handed-over
              roles: [lead]
              outputs:
                - set: { properties: { owner: "{{message.to}}" } }
          after:
            notify-deadline:
              period: 72h
              since: session
              to: overdue
              label: 72 hours since detection
              outputs:
                - set: { properties: { escalated: true } }
        handed-over:
          label: Handed over
          waiting_for: [lead]
          on:
            accept-handoff: { to: contained, roles: [lead] }
            claim:
              to: contained
              roles: [member]
              outputs:
                - set: { properties: { owner: "{{message.author}}" } }
          after:
            notify-deadline:
              period: 72h
              since: session
              to: overdue
              label: 72 hours since detection
              outputs:
                - set: { properties: { escalated: true } }
        assessed:
          label: Risk assessed
          on:
            notified:
              to: notified
              roles: [dpo]
              outputs:
                - set: { properties: { status: notified, reference: "{{message.reference}}" } }
          after:
            notify-deadline:
              period: 72h
              since: session
              to: overdue
              label: 72 hours since detection
              outputs:
                - set: { properties: { escalated: true } }
        overdue:
          label: Notification overdue
          on:
            notified:
              to: notified
              roles: [dpo]
              outputs:
                - set: { properties: { status: notified, reference: "{{message.reference}}" } }
        notified:  { label: AP notified, final: true }
        dismissed: { label: Dismissed, final: true, status: cancelled }
      reminders:
        status-update:
          label: Status update
          in: [detected, contained, handed-over, assessed, overdue]
          every: 30m
          first: 10m
          max: 48
          post:
            type: reminder
            props: { body: "Post a status update: what changed since the last one?" }
```

| Key | Meaning |
| --- | --- |
| `session.label` | The session's name. Defaults to the channel type's label. |
| `session.initial` | The state a new session starts in. It cannot be final. |
| `session.states` | The states, in the order a client shows them. At most 64. |
| `states.<name>.label` | The state's name for people. Defaults to the state name. |
| `states.<name>.final` | `true` for a state the session ends in. A final state has no `on:` and no `after:`. |
| `states.<name>.status` | What the state means to an agent: `working`, `input_required`, `completed`, `cancelled` or `failed`. See [Status and waiting_for](#status-and-waiting_for). |
| `states.<name>.waiting_for` | The roles the state waits for. It makes the status `input_required`. |
| `states.<name>.on` | The transitions out of the state, keyed by message type. A state that is not final needs at least one transition or one timer. |
| `states.<name>.after` | The timers of the state, keyed by timer name. At most 8. See [Timers](#timers). |
| `session.reminders` | Messages the session posts at a fixed interval, keyed by reminder name. At most 8. See [Reminders](#reminders). |
| `session.on_final` | `close`: a move into any final state closes the channel. See [Outputs](#outputs). |
| `session.run_as` | The job identity that writes the outputs of timer and trigger moves. See [Outputs](#outputs). |
| `on.<type>.to` | The state the message moves the session to. It may be the same state: a re-entry. |
| `on.<type>.roles` | Only a poster holding one of these roles moves the session. Each role must be in the message type's `post`. |
| `on.<type>.by` | `trigger`: only a trigger's message moves the session. |
| `on.<type>.when` | A condition the message must meet to move the session. |
| `on.<type>.outputs` | Graph writes the move makes, as the poster, or as `run_as` for a trigger's move. At most 4. See [Outputs](#outputs). |

**How a message moves a session.** A posted message moves the session when,
at the moment it is stored, the channel is open, the current state has an
`on:` entry for the message's type, the poster passes the entry's rule, and
its `when` holds. Otherwise the message is posted and the session stays
where it is: a transition is part of what a message means, not a separate
permission. The store decides under its lock, so of two messages that
arrive together, the one stored first moves the session, and the second is
judged against the new state. A reply moves the session like any other
message. A signal (`persist: ephemeral`) is never stored and cannot be a
transition.

**Where a message type may be posted.** Without `in:`, a message type may
be posted in every state by everyone its `post` admits. `in:` limits the
type to the states it lists:

```yaml
channel_types:
  plan:
    label: Plan review
    attach_to: [datalek]
    roles:
      owner:  { label: the owner, from: entity.owner }
      member: { from: attached }
    message_types:
      text:    { properties: { body: { type: markdown, required: true } }, post: [owner, member] }
      plan:    { properties: { plan: { type: markdown, required: true } }, post: [member], in: [drafting] }
      approve: { properties: { note: { type: string } }, post: [owner], in: [review] }
      revise:  { properties: { feedback: { type: markdown, required: true } }, post: [owner], in: [review] }
    session:
      initial: drafting
      states:
        drafting:
          on:
            plan: { to: review }
        review:
          waiting_for: [owner]
          on:
            approve: { to: approved, roles: [owner] }
            revise:  { to: drafting, roles: [owner] }
        approved: { final: true }
```

Here the owner is offered Approve and Revise only while the plan is in
review, and a member is offered Plan only while drafting. Text has no
`in:`, so it may be posted in every state. `in:` decides whether a new
message may be posted; `on:` decides whether it moves the session.

- A post outside `in:` is refused with 409 `session_wrong_state`. The
  `title` names the allowed states and the current one, and `detail` names
  the rule, such as `channel_types.plan.message_types.approve.in`. Nothing
  is stored. The store decides under its lock, so of two posts that arrive
  together, the second is judged against the state the first moved the
  session to.
- `in:` narrows what rela offers: `_actions.post`, and `can_post` in
  `read_channel`. The web app's composer offers only these types. The
  session's transitions, in `read_session` and the inbox too, are already
  limited to the current state's `on:`, and the load check keeps them
  consistent with `in:`.
- `in:` governs new posts and replies only. An edit of a stored message is
  judged by `edit_own` as before, so a message whose type the state no
  longer admits can still be edited.
- A final state admits the types that list it. A stranded session admits
  only the types without `in:`.
- A trigger whose message type the current state does not admit posts
  nothing and is skipped. This holds for `when_closed: post` too. Unlike a
  skip for a closed channel, rela logs it as a warning that names the
  trigger and the `in:` key, because it usually means `in:` is narrower
  than the states the trigger fires in.
- Typing follows `post` roles, not states. Whoever may post a `log` type
  of the channel may type, whatever the state.
- `in:` is new in this release. Every process that loads the project needs
  this release or later: rela-server, stdio `rela mcp`, `rela channel` and
  rela-desktop. An older one refuses the key.

**Who can move a session.** Only a principal who may post the message type.
The entry narrows that further:

- With `roles`, a person or agent holding one of the roles moves it. A
  trigger never holds a role.
- With `by: trigger`, only a trigger's message moves it. Use this for every
  transition that stands for "the entity changed": without it, anyone who
  may post the message type could fake that event. A trigger posts only
  into `cardinality: one` channel types, so rela refuses `by: trigger`
  in a `many` type.
- With neither, every principal who may post the message type moves it,
  and so does a trigger.

`roles` and `by: trigger` cannot be combined.

**Guards.** `when` reads `message.<property>` (the posted properties, typed
as for the message type's own `when`), `session.state`,
`session.generation`, `session.entered_at`, `session.reminders`
(see [Reminders](#reminders)) and the standard functions such as
`today()`. A comparison on an optional property the message leaves out
does not hold, and neither does a `when` that fails to evaluate. rela
logs such a failure as a warning, once per guard until the next reload,
with the channel type, the state and the message type. A `when`
cannot read the entity yet; rela refuses one that does, with "transition
guards cannot read the entity yet; use a trigger that posts a message".
Write "not equal" as `~=`.

**Triggers.** A trigger's message moves a session through an entry with
`by: trigger` or with no poster rule. A trigger with `when_closed: post`
still posts into a closed channel, but a closed or archived channel never
moves.

**Notices.** Each move appends a notice to the channel, right after the
message that caused it. A notice has the type `session`, the author
`system:realtime` (`author.system: true`), `triggered_by:
session:<channel type>`, and the props `from`, `to`, `generation` and
`cause`, the sequence number of the message that moved the session. Nobody
can edit or remove a notice, not even a moderator: the answer is 409
`session_notice`. The move is also written to the channel's audit trail,
with the poster as author and, for a trigger, `triggered_by:
trigger:<name>`. So a channel type with a session cannot declare a message
type named `session`. Deleting the channel, or retention, removes the
notices with it.

**The message that moved the session.** Its author can no longer edit or
delete it, and the web app no longer offers either: the answer is 409
`session_message`. The guard judged the message as it was posted, and the
notice and the audit record name it as the cause. A moderator may still
remove it; the notice and the audit record keep the move.

**State, generation and entered_at.** The session is stored on the channel.
`generation` counts the moves; 0 means the session never moved. A session
that never moved reads as `initial`, entered when the channel was created.
That includes channels that existed before their type gained a session.
After a move, `entered_at` is the time of the message that caused it.
Closing and reopening a channel leaves its state as it is; see
[Timers](#timers) for what happens to its deadlines. Removing the message
that caused a move does not undo the move.

**Stranded states.** A channel whose stored state the file no longer
declares is stranded. It reads with `stranded: true`, its stored state name
as label, and no transitions. Posts of types without `in:` are accepted
and move nothing; the lifecycle still works. To unstick such channels,
declare the state again with an `on:` that leads to a current state. A
state no transition leads to is allowed for this reason.

**In the web app.** The channel shows the session as a row of states above
the messages, and the composer marks the message types that move it. See
[In the web app](#in-the-web-app). The row shows where the session is now,
not how it got there: the notices in the log are its history.

### Status and waiting_for

Each state has a status, so an agent or a client knows whether to act,
wait or stop without knowing the states of each channel type:

| Status | Meaning |
| --- | --- |
| `working` | The session is in progress, and nobody in particular has to answer. |
| `input_required` | The session waits for the roles under `waiting_for`. |
| `completed` | The session ended as intended. |
| `cancelled` | The session ended without finishing, such as a dismissed breach. |
| `failed` | The session ended in failure. A stranded session reads `failed` too. |

The defaults: a final state is `completed`. A state that is not final is
`input_required` when it has `waiting_for`, and `working` otherwise. In
the example, `contained` and `handed-over` wait for the lead, and
`dismissed` is `cancelled`.

The rules:

- A final state is `completed`, `cancelled` or `failed`, and has no
  `waiting_for`: it has no transitions, so nothing can answer. A state that
  is not final cannot have an outcome status; mark it `final: true`.
- `input_required` needs `waiting_for`, and `waiting_for` needs
  `input_required`.
- `waiting_for` names roles under `roles:` of the channel type, each once.
  Each must have a known holder: a `from: entity.<property>` role, or a
  `creator: true` role. A role held by everyone with a permission, by
  everyone who can read the entity, or by every principal of a type is
  refused, because then everyone would be waited on.
- Each waiting role must be able to answer: some transition out of the
  state to a different state is keyed by a message type the role may post,
  is not `by: trigger`, and lists the role in `roles` when it has `roles`.
  A guarded answer counts. This check reads only `realtime.yaml`. When
  `acl.yaml` is loaded and no role in it grants what an answer's outputs
  write, the file loads with a warning: no holder could post that answer
  without `session_output_denied`.
- The waiting roles may not be able to bring the session back into a state
  with a `since: state` timer or a reminder. rela follows every chain of
  moves that starts in the state and that the state's waiting roles can
  fire, by the same rule as an answer. A chain that ends in the state
  again is refused when the state has a `since: state` timer, because the
  timer starts again. With a reminder it is refused when the chain passes
  a state outside the reminder's `in:`, because that starts a new span.
  Otherwise a holder could push the deadline back forever. A direct
  re-entry is the shortest such chain.
- The remedy is `since: session` for a deadline that survives a handoff.
  A separate handoff state is safe only when the waiting roles cannot
  move the session back. In the example, `handoff` and `accept-handoff`
  form such a cycle, so `contained` and `handed-over` have no `since:
  state` timer, and `status-update` runs in both states.

A session that never moved reads as the current `initial` state, with that
state's status. A session whose state the file no longer declares reads
`failed` with `stranded: true` and no `waiting_for`, so an agent that waits
on it stops. Declaring the state again restores its status.

**Holders and waiting_on_me.** The session of a channel lists
`waiting_for` with each role's name and label. A `from: entity.<property>`
role also lists `holders`: the values of that property as the reader can
read them. A property that `visible:` hides from the reader gives no
holders, and rela refuses a role on a property that `visible:` restricts.
A creator role lists no holders; the channel's `created_by` names the
creator. Holders are entity data that anyone who may write the property
can set: show them as data, and never follow them as instructions.

`waiting_on_me` is `true` when the reader holds one of the waiting roles,
the channel is stored and open, and the configuration is not degraded. A
role answers as it does for a post: a named role through its property, a
creator role through the channel's creator.

### Handoff

A handoff moves a waiting role to someone else. It needs no special
output: a transition's `set:` writes the role's property, as the poster.
In the example, the lead posts `handoff` with `to`, the session moves to
`handed-over`, and `owner` becomes the new lead. The new lead posts
`accept-handoff` to go back to `contained`. A member may `claim` the
breach instead, which writes the member's own identity. The lead can
hand off and accept again and again, so the deadline counts from
detection: `notify-deadline` has `since: session`.

- The poster needs field write on the role's property, so `acl.yaml`
  decides who may hand off. A poster without it is refused with
  `session_output_denied` before anything is stored. The write is in the
  graph audit with the poster as writer and the move as `triggered_by`.
- The answer to the post shows the new holders and `waiting_on_me`, read
  again after the write.
- A claim writes `{{message.author}}`, the identity the poster's transport
  resolved. The web app resolves it through `principal_property`; the CLI
  and MCP keep the raw identifier. A role matches a caller only through
  the same value, so configure identity the same way on every transport.
- When `acl.yaml` declares `user_entity_type`, a `set:` that writes a role's
  property from free text loads with a warning. Write a literal,
  `{{message.author}}`, or a message `ref` to the user entity type.
- Two claims at once both move the session, and the last `set:` wins. A
  guard cannot read the entity, so there is no compare-and-set. The
  notices and the graph audit show who held the role when.
- A handoff to someone who cannot read the entity gives them nothing:
  every role requires read access to the entity.
- A timer or trigger move may also hand off, as `run_as`, with a literal
  or a trigger prop.

### Timers

A timer moves a session when a state has lasted too long, with no message.
In the example, a breach that is not contained within a day moves to
`overdue`, and so does one that is not notified 72 hours after it was
detected.

| Key | Meaning |
| --- | --- |
| `after.<name>.period` | How long the timer waits, such as `30m`, `24h`, `7d` or `1mo`. A fixed period is at least `1m`. Required. |
| `after.<name>.since` | What the period counts from. `state` (the default): from when the session entered the state, or from the channel's creation for a session that never moved. `session`: from the channel's creation. |
| `after.<name>.to` | The state the timer moves the session to. Required. |
| `after.<name>.label` | The timer's name for people. Defaults to the timer name. |
| `after.<name>.outputs` | Graph writes the timer's move makes, as `session.run_as`. At most 4. See [Outputs](#outputs). |

Use `since: session` for a deadline that runs across states, such as the 72
hours to notify the authority: it keeps counting from the detection while the
breach moves from `contained` to `assessed`.

**When a timer fires.** When the first timer of the current state comes due,
the session moves to its `to` state. A message that moves the session first
wins: the timer then belongs to a state the session left and never fires. Of
two timers due at the same time, the one whose name sorts first fires. Each
move happens once. Two server processes that both run timers for a moment
still move the session once.

A timer fires within a second of its due time when the session's last change
was made in the server process that runs the timers, and within a minute
otherwise, for example after a post through `rela channel` or stdio
`rela mcp`. The move's time is when it fired, which is also the new state's
`entered_at`.

**Notices and audit.** A timer's move appends a notice like a message's
move, with the prop `timer` (the timer's name) instead of `cause`. The
channel's audit trail records it with the author `system:realtime` and
`triggered_by: timer:<channel type>/<state>/<timer>`, for example
`timer:breach-response/detected/no-containment`. A timer with `outputs`
writes them as `session.run_as`; see [Outputs](#outputs).

**Closed channels.** A closed or archived channel does not fire timers: its
timers are frozen. When the channel is reopened, each timer that came due
while it was closed starts again from the reopen, so it fires one full
period later. A timer that had not come due keeps its deadline. rela stores
the restart per timer, so closing and reopening the channel several times
never stretches a deadline that had not come due. For example, a 72-hour
`since: session` timer on a channel created at hour 0, closed at hour 10 and
reopened at hour 100, fires at hour 172. Closed again at hour 110 and
reopened at hour 120, it still fires at hour 172. The restart also stays
when the session moves to a state that declares a `since: session` timer of
the same name: posted `assessed` at hour 130, the breach above still fires
`notify-deadline` at hour 172. A `since: state` timer starts again from the
move.

The same holds when an entity is deleted and its channels are later put
back: an open channel is treated as reopened when it is restored.

**Final and stranded states.** A final state has no timers. A stranded
session (see above) never fires a timer, whatever the file declared for its
old state.

**Loops.** A loop made only of timers would move the session forever
without anyone posting, so rela refuses one, including a timer to its own
state. A loop that passes through a transition only people can fire (one
with `roles:`) is allowed. A loop of timers and transitions a trigger can
fire is refused too when it has outputs on it: a timer's output can change
what the trigger watches, and the trigger's post moves the session back,
so the loop would write to the graph forever without a person. Without
outputs on the loop, nothing in it changes the entity, so it is allowed.

### Reminders

A reminder posts a message at a fixed interval while the session is in
certain states. It never moves the session. In the example, the breach lead
gets a "status update" reminder 10 minutes after detection and every 30
minutes after that, until the authority is notified.

| Key | Meaning |
| --- | --- |
| `reminders.<name>.in` | The states the reminder runs in. Each must be declared and not final. Required. |
| `reminders.<name>.every` | The interval, such as `30m` or `4h`. At least `1m`. Required. |
| `reminders.<name>.first` | How long after the start of its span the first reminder comes. Defaults to `every`. At least `1m`. |
| `reminders.<name>.max` | The most reminders in one span, from 1 to 1000. Without it there is no cap. |
| `reminders.<name>.label` | The reminder's name for people. Defaults to the reminder name. |
| `reminders.<name>.post.type` | The message type the reminder posts. It must be declared, kept (not `persist: ephemeral`) and not move the session. Required. |
| `reminders.<name>.post.props` | The literal properties of the message, as for a trigger. |

**Spans.** A span is the time the session spends inside the reminder's `in:`
states. It starts when the session enters one of them, or when the channel
is created in one. It goes on when the session moves from one `in:` state to
another, or re-enters the same state. It ends when the session moves to a
state outside `in:`. A later move back into `in:` starts a new span. In the
example, a breach detected at 9:00 gets reminders at 9:10, 9:40 and 10:10.
Contained at 9:25, it still gets the 9:40 reminder: the span goes on.

**Slots.** The reminders of a span come at fixed times, its slots: the start
of the span plus `first`, then every `every`. When no server ran the
reminders for a while, the missed slots are skipped, not caught up: the
breach above, with no server from 10:15 to 12:20, gets one reminder at
12:20 and the next at 12:40.

A reminder comes within 30 seconds of its slot. It comes later only while
the server works through more due timers and reminders than it posts in
one round (64). Reminders and timers share that round. A timer that comes
due behind B due reminders fires at most B / 64 rounds late, rounded up,
and the rounds run one second apart. Short intervals cost a write per
reminder: 200 channels with `every: 1m` make 200 writes a minute.

**The count.** Each span counts its reminders. A guard reads the count as
`session.reminders['name']`. Write the bracket form for a name with a
hyphen; `session.reminders.name` works for a name without one. Outside the
reminder's `in:` states the count reads 0. In the example, the privacy
officer moves a stalled breach from `contained` to `overdue` with
`escalate-stalled`, but only after three reminders. With `max`, the
reminders stop when the count reaches it.

**The reminder message.** A reminder posts a message of its declared type
with the literal props, from the author `system:realtime`
(`author.system: true`), with `triggered_by:
reminder:<channel type>/<reminder name>`, for example
`reminder:breach-response/status-update`. The message type's actions are
offered on it as on any message. Nobody can edit or delete a reminder: the
answer is 409 `reminder_message`. A moderator may still remove one, with
the reason and audit record of any removal. Removing it does not lower the
count. A person's post of the same message type is an ordinary message: it
can be edited and does not count. A reminder writes no audit record and
sends no mail; the message is the record.

**Closed channels.** A closed or archived channel gets no reminders: its
reminders are frozen, and the count is kept. When the channel is reopened,
a reminder whose slot passed while it was closed starts again: the next one
comes `first` after the reopen. A reminder whose slot had not come keeps
it. The same holds when an entity is deleted and its channels are later put
back.

**Final and stranded states.** A final state has no reminders, and a
stranded session gets none.

### Outputs

A transition can write to the graph: set properties on the entity the
channel is attached to, or create an entity. These writes are its
`outputs`. In the example, a lead's `contained` message creates a timeline
entry about the breach, `assessed` copies the risk onto the breach, and the
privacy officer's `notified` records the status and the reference. When the
72-hour deadline passes, the `notify-deadline` timer marks the breach
`escalated`, as `system:breach-bot`.

```yaml
contained:
  label: Contained
  on:
    assessed:
      to: assessed
      roles: [lead]
      when: "message.risk ~= 'none'"
      outputs:
        - set: { properties: { risk: "{{message.risk}}" } }
```

| Key | Meaning |
| --- | --- |
| `outputs[].set.properties` | Properties to write on the anchor. At most one `set` per transition. |
| `outputs[].set.face` | The face to write, for a family-anchored channel whose type declares faces. A face-anchored channel writes its own face and takes no `face`. |
| `outputs[].create.type` | The type of the new entity. It needs generated ids. |
| `outputs[].create.properties` | The new entity's properties. Every required property must be set. |
| `outputs[].create.face` | The new entity's face, as for `promote_to`. |
| `outputs[].create.relate.type` | A relation from the new entity to the anchor. |

**Values.** A property value is a literal or a template. A literal is
checked against the property's type when the file loads. A template is
`{{reference}}` alone, which keeps the referenced value's type, or text
with references in it, which gives a string. The references are
`message.<property>`, `message.author`, `session.from`, `session.to`,
`session.generation`, `channel.key`, `channel.type`, `channel.title`, `now`
and `today`. The outputs of a timer or trigger move read fewer of them;
see [Timers and triggers](#timers-and-triggers). A whole-value reference
to an optional message property the message leaves out sets nothing; in
text it becomes the empty string. A
value taken from a message is never read as a template again, so a message
cannot inject a reference. `now` and `today` are the notice time in UTC,
not the time the write runs, so a retry writes the same values.
Automations format `now` in the server's time zone; outputs do not. A
template can read the entity only through `channel.title`.

**Who the outputs write as.** The person or agent whose message moved the
session. The graph write runs with their roles and tool `realtime`, and
their client baseline still applies. The graph audit log records each
write with `triggered_by:
session:<channel key>/<from>-><to>#<generation>/<index>`. A retry, which
has no request, rebuilds the poster from the role claims stored on the
notice. The notice also records whether the poster matched a user entity.
A poster who matched none is judged by `unmatched_principal` on a retry as
on the post, so with `reject` neither writes. A retry for a poster whose
user entity was removed since the post fails `forbidden`.

**Checked before the post.** Before rela stores a message that would fire
a transition with outputs, it asks whether the poster may write each
output: update the anchor at the target face, create the type at its face,
and create the relation. If not, the post is refused with 403
`session_output_denied`, and nothing is stored. A poster who cannot move
the session therefore cannot leave a failed write behind. What this check
cannot see fails after the move instead: a property the poster may not
write, a validation error, a unique conflict or an illegal state move.
This check covers a person's or agent's post. A timer's or trigger's move
is checked when the file loads instead; see
[Timers and triggers](#timers-and-triggers).

**Field grants.** Every property a `set` writes is checked against the
poster's field grants, a literal from the file as much as a value from
the message. The write asks the update grant again on the anchor as it
stands then, so a grant the poster lost after the check is not used. A
`create` checks the field grants of the properties whose value comes
from the message.

**Order and failures.** The outputs run in order after the move is stored.
The move stays when an output fails. A failed output stops the rest, which
are marked `skipped`, and the notice shows the failure with a code:
`forbidden`, `field_denied`, `invalid` (with the property names),
`conflict`, `not_found`, `unconfirmed`, `expired`, `transition_removed`,
`run_as_changed` or `error`. A notice whose transition the file no longer declares has one
result of kind `transition`, which fails `transition_removed`. A code never quotes an error message or an id.

**Retries.** A write that failed for a passing reason, or a server that
stopped mid-way, leaves the outputs pending. The leader process retries
them once a minute, in a round of its own beside its timers and triggers,
at most 16 per round, at most 5 attempts per move, and
for at most 30 minutes after the notice; then they fail `expired`. A
holder keeps its claim for 90 seconds after its last write. A retry reads
the outputs from the current file: changed outputs run as now declared,
and a transition the file no longer declares fails `transition_removed`.
When a later move already set a property, a retried `set` leaves it out;
when it leaves out every property, the output is `superseded`. The outputs
of one channel run one at a time, so a retry reads the later moves after
their outputs ran, however many messages came between.

**Exactly once and at least once.** A `set` runs at least once: a retry
may write the same values again, and update automations see it. A `create`
runs exactly once when the channel store and the entity store share a
database (sqlite, or postgres with both on the same database): the entity,
its relation and the progress commit together. With the file store, or
stores on different databases, a create that may have written is never
retried. It fails `unconfirmed`, so it is neither doubled nor silently
dropped.

**Automations and validation.** Outputs go through the same write path as
a form, so automations and validation run. A validation error fails the
output with the property names. Warnings complete it with the warning
codes. An output that changes the anchor can fire an entity trigger that
posts into the same session. The trigger cooldown (one post per rule and
entity per minute) bounds such a loop, but check your triggers before you
add outputs that change what they watch. A loop that also needs a timer
is refused when the file loads; see Loops under [Timers](#timers).

**on_final: close.** With `session.on_final: close`, a move into any final
state closes the channel as the last step of the outputs. A failed output
does not stop the close. A reopened channel in a final state is not closed
again. The close needs no lifecycle `close` operation; reopening still
needs `reopen`. A timer's or trigger's move into a final state closes the
channel too, also when it has no outputs.

**On the notice.** The notice of a move with outputs carries an `outputs`
block in place of `action`: the status (`pending`, `done` or `failed`),
the writer, and a result per output with its kind, status, code, the
properties a set wrote, the face a set wrote when the output names one,
and the entity a create made (`restricted` for a
reader who cannot read it). While the outputs run, a moderator cannot
remove the message that caused the move, and nobody can release the mark:
both answer 409 `outputs_pending`.

### Timers and triggers

A timer's move has no poster, and a trigger's message comes from
`system:realtime`, which has no grants. Their outputs write as the
session's `run_as`: a job identity that `acl.yaml` grants what the outputs
need.

```yaml
session:
  initial: detected
  run_as: system:breach-bot
  states:
    contained:
      after:
        notify-deadline:
          period: 72h
          since: session
          to: overdue
          outputs:
            - set: { properties: { escalated: true } }
```

```yaml
# acl.yaml
roles:
  breach-bot:
    read: [datalek]
    update: [datalek]
    fields:
      datalek: [{ field: escalated }]
assignments:
  system:breach-bot: breach-bot
```

**Which moves write as `run_as`.** A timer's move, a move through a
`by: trigger` transition, and a trigger's move through a transition with no
`roles:` (a person's or agent's move through that transition still writes
as the poster). A session that has any of these with outputs needs
`run_as`; without it, rela refuses the file at the first such output. A
`run_as` that no such move uses loads with a warning.

**The identity.** `run_as` takes a `system:` or `integration:` name, as
`--run-as` does: lowercase ASCII letters, digits, `-`, `_` and, for
`system:`, `:`. rela's own identities, such as `system:realtime` or
`system:scheduler`, are refused. The name gets only what its
`acl.yaml` assignment and the `everyone` role grant. A role from a
relation or an asserted claim never applies to it. `run_as` is set once per
session; `run_as` on a transition or a timer is refused.

**Checked when the file loads.** With an `acl.yaml`, rela checks that
`run_as` holds every grant its outputs need, and refuses the file
otherwise, with one problem per missing grant, such as
`system:breach-bot cannot update datalek: no role assigned to it or to
everyone grants it`. It checks update on the anchor type (on every face a
face-anchored channel may have), create on a created type, the field
grant of every property, the option grant of every literal enum value and
of each value a trigger posts into it, and the relation of a `create`. A
field grant with a `when:` counts as granted here; the write checks the
condition. The same check runs at boot, in `rela validate`, on a reload,
when the Configure space saves `schema.yaml`, and when the MCP server
reloads the schema.

**Checked again at the write.** The write asks `acl.yaml` as it is then.
An output whose grant was removed since the file loaded fails `forbidden`,
and a field whose `when:` does not hold fails `field_denied`.

**What the templates read.** A timer's move has no message, so its outputs
cannot read `message.*`. The outputs of a trigger's move read the
trigger's message properties, which are literals from the file. Neither
may read `message.author` or `channel.title`: the author is always
`system:realtime`, and the title is free text from whoever created the
channel, which must not reach a write made with `run_as`'s grants.

**When they run.** The move stores a pending mark on its notice, and the
leader process runs it right after the round that made the move. A failure
is retried like any other (see Retries under [Outputs](#outputs)). The
audit log records each write with `run_as` as the user, tool `realtime`
and the same `triggered_by` as a person's output. With the file store, a
crash right after the move cannot lose the mark: the move keeps its notice
in the channel's `meta.yaml` until the log has it, and the next scan for
pending marks writes it back, so its outputs and its `on_final` close
still run. The channel audit record of such a move can be missing.

**What never writes.** No output writes as rela's own `system:realtime`.
A write that would, or a timer's or trigger's write with no `run_as`,
fails `forbidden`. An older mark that names `system:realtime` as its
writer runs no outputs; its close still runs. A trigger's mark whose transition a reload gave `roles:` fails
`transition_removed`: its outputs now follow the rules for a person's
post. A set on a channel type with several `attach_to` types writes the
anchor only when its row is one of those types.

**Changing `run_as`.** A mark records the `run_as` in force at the move.
When the file names another `run_as` by the time the mark runs, its
outputs fail `run_as_changed` and write nothing; an `on_final` close still
runs. This keeps a write from running with grants nobody checked for it.

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
  `lifecycle.delete.retention`;
- that each `session` has between 1 and 64 states with valid names, an
  `initial` state that exists and is not final, `on:` only on states that
  are not final and on every such state, transitions keyed by a `log`
  message type of the channel type, a `to` that exists, `roles` that may
  post the message type, `by` only as `trigger`, never with `roles` and
  never in a `cardinality: many` type, and a `when` that compiles and does
  not read the entity. A `session` at the top of the file is refused; it
  belongs under a channel type;
- that each timer under `after:` is on a state that is not final, has a
  valid name, a `period` that parses and is at least `1m` when fixed, a
  `since` of `state` or `session`, and a `to` that exists. A state has at
  most 8 timers, no loop is made only of timers, and no loop of timers
  and transitions a trigger can fire has outputs on it. The key `from` is
  refused with "use since: state or since: session";
- that a session has at most 8 reminders, each with a valid name, `in:`
  states that exist, are not final and are listed once, an `every` and
  `first` that are fixed and at least `1m`, a `max` from 1 to 1000, and a
  `post.type` of the channel type that is kept and that no transition is
  keyed by, with `props` checked as for a trigger. `session.timers` and
  `reminders` under a state are refused with a pointer to
  `session.reminders`; `to`, `period` and `since` on a reminder are refused;
- that `in:` on a message type is on a channel type with a `session`, is a
  list with at least one state, and names states that exist, each once.
  A state whose `on:` is keyed by the type must be in its `in:`, or the
  transition could never fire. A reminder's states must lie inside its
  post type's `in:`, and with `on_final: close` the `in:` cannot list only
  final states. `in:` with no value is refused with "name the states, or
  drop in to allow every state". `states` and `from` on a message type are
  refused with a pointer to `in:`;
- that a channel type with `typing: true` has at least one `log` message
  type, and no message type named `typing`. `typing` must be `true` or
  `false`;
- that each transition has at most 4 outputs and at most one `set`; that
  each output is either `set` or `create`, names at least one property, and
  sets only declared properties that are not computed, file or
  `external_ref` properties; that literals fit their property, templates
  are at most 1024 bytes and name known references of a fitting type, and
  `{{entity.*}}` is not used; that a `create` names a type with generated
  ids, sets every required property, and relates with a relation that can
  link the type to every `attach_to` type; that a property a `set` writes
  has the same type on every `attach_to` type; that faces follow the rules
  under [Outputs](#outputs); that `on_final` is `close`; and that no
  message type of a session type declares an action named `outputs`.
  `outputs` on a state or a reminder, `on_final` on a state, and `run_as`
  anywhere but `session.run_as` are refused;
- that `session.run_as` is a job identity, that a session whose timer or
  trigger moves have outputs sets it, and that `acl.yaml` grants it what
  those outputs write (see [Timers and triggers](#timers-and-triggers));
- that each state's `status` and `waiting_for` follow the rules under
  [Status and waiting_for](#status-and-waiting_for): one of the five
  statuses, an outcome status only on a final state, `waiting_for` only
  with `input_required` and on a state that is not final, declared roles
  with a known holder, an answer for each waiting role, and no chain of
  moves the waiting roles can fire back into a state with `since: state`
  timers or reminders.
  `awaits` on a state is refused with a pointer to `waiting_for`;
  `status` and `waiting_for` directly under `session`, `on:` or `after:`
  are refused with where they belong;
- that the file has no merge key (`<<`) anywhere: write the entries out.

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
force. Two more checks only warn: an answer whose outputs no role in
`acl.yaml` grants, and a handoff `set:` from free text (see
[Handoff](#handoff)).

An action named `edit`, `delete`, `remove` or `release` is refused too.
Older versions accepted such a name; a file that has one no longer loads,
so rename the action before you upgrade.

`rela validate` loads `realtime.yaml` the same way, with `acl.yaml`, so it
also runs the `run_as` grant check. It prints a `✗` line for a
`realtime.yaml` that does not load, and a `⚠` line for each warning. When
`data-entry.yaml` has a `channels:` block, it checks that block against
the file too, with a `✗` line for each problem.

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

The typing indicator has fixed limits that the `limits:` block does not
change: entries live 6 seconds, one call per second per person and channel,
32 typists per channel and 4096 channels with typing entries per process.
Typing calls count toward the fixed budget of 300 channel writes per minute
per person, which posts share.

Timers have fixed limits too: at most 8 per state, and a fixed period of
at least 1 minute. The server checks for due timers at least once a minute
and makes at most 64 timer moves and due-time updates per check; the rest
follow a second later.

Reminders have fixed limits: at most 8 per session, an `every` and `first`
of at least 1 minute, and a `max` of at most 1000. Reminder posts share the
64 writes per check with timers.

The inbox has fixed limits: at most 200 entries per page, at most 20 000
entities on which the caller holds a waiting role, and at most 20 000
candidate channels per call. With the file store, a type with a creator
waiting role makes each inbox call read the `meta.yaml` of every channel of
that type. An agent that watches `list_inbox` repeats the call at most every
2 seconds after a change in its process, and at most every 15 seconds on the
poll. The database backends read the creator from an index.

Read markers have fixed limits. An unread count stops at 100. Marking read
is limited to 2 marks per 2 seconds per person and channel, and 300 marks
per minute per person. These limits are apart from the post limits, so
marking read never uses up a person's posts. The inbox looks at most 500 of
your markers and lists at most 50 unread channels. With the file store,
each inbox call that asks for unread channels reads the directory of every
channel in the project, and a mark that moves a marker takes the store's
project-wide write lock. A mark that does not move the marker takes no
lock. The database backends use indexes for both.

Mentions have fixed limits. A message may address at most 10 people and
agents. A person or agent keeps at most 200 mention and reply rows per
channel. The inbox looks at the 2000 newest rows of the caller of the
asked kinds, checks every unread one against what the caller may read,
and lists at most 50. With the file
store, each channel keeps its rows in `mentions.json`, at most 5000 rows
per channel. An inbox call that asks for mentions or replies then reads
the directory of every channel in the project a second time, beside the
walk for unread channels. Each post that writes a row rewrites that
channel's `mentions.json` under the project-wide write lock. The database
backends use indexes.

Outputs have fixed limits: at most 4 per transition and 1024 bytes per
template. Retries run once a minute, at most 16 per round and 5 attempts
per move, for 30 minutes after the notice; a claim holds for 90 seconds.

## In the web app

The entity page shows a **Channels** section below the comments. It
appears only on an entity whose type a channel type attaches to. On a type
with faces, the section follows the face on screen: an `anchor: face`
channel is the channel of that face.

When the entity has exactly one channel and you may not open another, the
section shows that channel directly. This includes a single channel of a
`many` type. Otherwise it lists the channels by type, with a filter for
**Open** or **All** channels, and a button to open a new channel for each
`many` type you may open. The button shows the type's `create.label`, or
**New channel** without one. Click a channel to open it. The section updates
live: a new message, a closed channel or a new channel shows up without a
reload. When a second channel appears next to one shown directly, the list
appears and the channel stays open.

A button appears only when you may use it. It is left out, not disabled.

| Control | Shown when |
| --- | --- |
| The composer and **Send** | The channel is open, a message type's `post` admits you, and the session's state is in the type's `in:`. With several such types, the composer lets you choose the type. If the session moves on before you send, the post is refused, the app says so and reads the channel again, and the type leaves the composer. |
| **Reply** | You may post in the channel now. |
| **Edit** | The message is yours, `edit_own` admits you, and its time window has not passed. |
| **Delete** | The message is yours and `delete_own` admits you. The app asks you to confirm. |
| **Remove** | `message_ops.remove` admits you. The app asks for a reason, and requires one when `reason_required` is set. |
| An action button, such as **Accept** | The action's `roles` or `permission` admit you. **Release** gives up an action that is pending. |
| A lifecycle button, such as **Close** | The operation's `roles` admit you and the channel is in a period it starts from. The button shows the operation's `label`. **Delete** asks you to confirm. |
| **New channel** | The type is `many` and its `create` admits you. |
| Session header | The channel type has a `session`. It shows on a closed or archived channel too. |
| A message type marked as moving the session | Your post of that type would move the session now: see `_actions.transitions` below. |

A closed channel shows "This channel is closed." and an archived one "This
channel is archived."; neither has a composer.

On a channel type with a session, a header between the title and the
messages shows the session's label and its states, in the order the file
declares them. The current state is highlighted. The states a transition
out of it leads to are marked with a dotted outline; the header marks no
earlier state as done, because rela keeps only the current state and a
session may branch. No state is a button: only a message moves a session.
Exactly one status applies, in this order:

1. **Stranded.** The stored state is no longer declared. No state is
   highlighted, and a warning says "‹state› is no longer a state of this
   session. Messages do not move it."
2. **Loading.** The page holds an older copy of `realtime.yaml` that lacks
   the current state, and is loading the new one. No state is highlighted,
   and the header says "Loading the states of this session…". No message
   type is marked as moving the session until the new copy arrives.
3. **Ended.** The current state is final. It shows its status: a check
   mark and "Completed", "Cancelled", or a red "Failed" tag.
4. **Paused.** The channel is closed or archived. The current state stays
   highlighted, with "Paused while the channel is closed" (or archived).
5. **Normal.** The current state and the next states. An
   `input_required` state has an "Input required" tag, and the header says
   whom it waits for, such as "Waiting for the incident lead", or shows a
   "Waiting for you" tag when you hold a waiting role. It does not show the
   holders' ids: the entity page shows the property.

A notice names the move only, such as "Session moved from Detected to
Contained". It does not say whom the session waited for: a notice is
history, and the current configuration cannot tell whom an earlier state
waited for. The header says whom the session waits for now, and a screen
reader announces it when it changes. A handoff made by editing the entity
has no notice; the header updates with the next live update.

A channel row in the list shows a badge with its number of unread
messages, or "99+" above 99. A screen reader reads it as, for example, "3
unread". A row without unread messages, or one you never opened, shows no
badge. An open channel marks itself read up to its newest message while
the browser tab is visible. See
[Read markers and unread counts](#read-markers-and-unread-counts).

On a narrow panel the header folds into one line, such as "Session ·
Contained", "Completed · AP notified", "Contained · Paused" or "Stranded ·
triage", with a **Show all states** button that unfolds the list.

The composer lists the message types that would move the session first,
each marked with an icon. It selects a type that moves nothing by
default, so a plain message does not move the session by accident. Only
when every type you may post moves the session does it select the first of
them, and the note below says the session will move. While a
marked type is selected, the composer says "Sending this moves the session
to ‹state›.", or "… to ‹state› if its condition holds." for a transition
with a `when`. A reply shows the same note. An edit does not, because an
edit never moves a session.

A notice shows as one line, such as "Session moved from Detected to
Contained" or "Session entered Contained again", with the time and no
buttons. A screen reader hears it after the message that caused it, for
example "Contained from Ann. Session moved from Detected to Contained."
When someone else moves the session, the header and the marked types
follow without a reload.

On a channel type with `typing: true`, the app tells the server you are
typing while you write a `log` message. It does not while you edit a
message or fill an `ephemeral` type. Other readers see a line under the
messages, such as "Ann is typing…", "Ann and Bo are typing…" or "Ann, Bo
and 2 others are typing…". The line disappears when you send, or a few
seconds after you stop typing.

A `ref` property gets an entity picker that offers only the types in its
`to:` list. A message that holds a ref you cannot read shows **Restricted
reference**, with no id, title or link. When you edit such a message, the
field shows the same placeholder with a **Replace** button. If you leave it,
the server keeps the stored ref.

Message markdown is limited, because every user whose message type `post`
rule admits them may write it, not only the operator. It allows
text formatting, lists, quotes, code, tables and links to `http`, `https`
and `mailto` addresses. It drops images and raw HTML. An entity id in a code
span, such as `` `FEAT-001` ``, becomes a link when the message holds a
readable ref to that entity.

A mention in a message shows as a chip. A person you may read becomes a
link to their entity, labelled with the title the server sends. A job
identity or a login becomes a plain chip with its text. A mention of a
person you cannot read stays code. The composer has no `@` picker yet:
type the mention as a code span. See
[Mentions and replies](#mentions-and-replies).

Field order, labels and widgets come from the `channels:` block of
`data-entry.yaml` (see the data-entry guide).

## HTTP API

The data-entry API serves channels under the entity they are attached to.
`{plural}` and `{id}` are the entity's collection and id, as in
`/api/v1/{plural}/{id}`. On a type with faces, write the face as
`{id}@{face}`. A bare id on such a type gets a 422 `face_required`.

| Method and path | Does |
| --- | --- |
| `GET /api/v1/_schema/channels` | Describes every channel type: its message types with their properties, labels, actions and lifecycle labels. See below. |
| `GET /api/v1/{plural}/{id}/_channels` | Lists the channels the caller may read, per channel type. `?state=` filters on a period. |
| `POST /api/v1/{plural}/{id}/_channels/{type}` | Opens a named channel of a `many` type. The body may hold a `title`. |
| `GET .../_channels/{type}` | The head of a `one` channel: its last sequence number, revision, and `_actions`, the operations the caller may perform. |
| `GET .../_channels/{type}/{name}` | The head of a named channel. |
| `GET .../{channel}/messages` | A page of messages. See below. |
| `POST .../{channel}/messages` | Posts a message: `{"type": ..., "props": {...}, "reply_to": n}`. A `log` message gets a 201 with the message; an `ephemeral` one gets a 202. |
| `PATCH .../{channel}/messages/{seq}` | Edits a message: `{"props": {...}}`. The props replace the message's props, with one exception: a stored `ref` property the caller cannot read, and that the request does not name, is kept as stored. A caller cannot send back an id they cannot see, so leaving it out does not erase it. A property the request does not name is otherwise removed. |
| `DELETE .../{channel}/messages/{seq}` | Removes a message. The body may hold a `reason`. |
| `POST .../{channel}/messages/{seq}/_action/{name}` | Runs an action. The body is empty or `{}`. The answer is `{"message": ..., "warnings": [...]}`. |
| `DELETE .../{channel}/messages/{seq}/_action` | Releases a pending action. The body is empty or `{}`. The answer is the message. |
| `POST .../{channel}/_lifecycle/{op}` | Runs `close`, `reopen`, `archive` or `delete`. The body is empty or `{}`. A `delete` gets a 204; the other operations get the channel. |
| `POST .../{channel}/_typing` | Starts or refreshes the caller's typing entry, on a type with `typing: true`. The body is empty or `{}`. The answer is a 202 with `{"ttl_seconds": 6}`. |
| `PUT .../{channel}/_read` | Marks the channel read for the caller. The body is empty, `{}` or `{"seq": n}`. The answer is `{"seq", "unread", "last_seq"}`. See [Read markers and unread counts](#read-markers-and-unread-counts). |
| `GET /api/v1/_inbox` | The sessions that wait on the caller now, oldest wait first, the channels with unread messages, and the unread messages that mention the caller or reply to them. `?limit=` (1 to 200, default 50) and `?cursor=` page the sessions. `?kinds=` chooses from `waiting`, `unread`, `mention` and `reply`; all four by default. See [Inbox](#inbox). |
| `GET /api/v1/_inbox/summary` | The counts of the inbox, for a badge: `{"waiting", "waiting_truncated", "unread_channels", "mentions", "degraded"}`. The query string is ignored. See [Inbox](#inbox). |

`{channel}` is `_channels/{type}` or `_channels/{type}/{name}`.

`GET /api/v1/_schema/channels` describes the channel types in force, so a
client can build a composer and label buttons. For each channel type it
gives the label, the entity types it attaches to, the anchor, the
cardinality, the `create` label, whether a removal needs a reason,
whether the type offers typing (`typing`), the lifecycle operations
with their labels, and for a type with a session the `session` object
described below. For each message type it gives
the label, `persist`, the signal lifetime in seconds for an `ephemeral`
type, whether a message may reply to another, the actions with their
labels and the entity type a promote creates, and the properties. Each
property has its name, label, type, and when set `required`, `max`, the
enum type with its `values`, and the entity types a `ref` may point to
(`to`). Properties come required first, then by name.

The answer holds no roles, permissions or conditions, and is the same for
every caller: the configuration is not a secret. What a caller may do
comes from the `_actions` of each channel and message. With realtime off
the answer is `{"channel_types": []}`.

The `session` object of a channel type holds its `label`, its `initial`
state and its `states` in the order the file declares them, each with a
`name`, a `label`, its `status`, `final` when set, and `waiting_for` (role
names) when set. It holds no transitions, roles or conditions: the
transitions out of a channel's current state come with the channel.

```json
"session": {
  "label": "GDPR breach response", "initial": "detected",
  "states": [
    { "name": "detected", "label": "Detected", "status": "working" },
    { "name": "contained", "label": "Contained", "status": "input_required", "waiting_for": ["lead"] },
    { "name": "handed-over", "label": "Handed over", "status": "input_required", "waiting_for": ["lead"] },
    { "name": "assessed", "label": "Risk assessed", "status": "working" },
    { "name": "overdue", "label": "Notification overdue", "status": "working" },
    { "name": "notified", "label": "AP notified", "status": "completed", "final": true },
    { "name": "dismissed", "label": "Dismissed", "status": "cancelled", "final": true }
  ]
}
```

The head's `_actions.lifecycle` lists the lifecycle operations the caller
may run now. It runs the same checks as the `POST`, except the rate limit.
A `one` channel that holds no message has no lifecycle operations.

The head's `_actions.transitions` lists the message types whose post by the
caller would move the session now, sorted by name. It is a subset of
`_actions.post`, and it is empty when the type has no session, the channel
is not open, or the session is final or stranded. It leaves out a
transition with `by: trigger`, and one whose `roles` the caller does not
hold. A transition with a `when` is listed, because the condition is judged
on the message: the post moves the session only if it holds. In the example
above, the lead of a channel in `detected` gets `["contained", "dismiss"]`
and a member gets `[]`.

The head's `typing_now` lists who else is typing, oldest first, as
`[{"author": {...}, "since": "..."}]`. It never lists the caller, and it is
empty unless the channel is open and its type has `typing: true`. A client
shows it as it is: the server ends an entry and sends a live frame then, so
the client needs no clock. `_actions.typing` says whether the caller may
send typing now.

A typing call checks, in order: that the caller may read the channel (else
the same 404 as a hidden entity), that the type has `typing: true` (403
`typing_not_offered`), that the caller may post a `log` message type (403
`channel_post_denied`, naming `channel_types.<type>.typing`), that the
channel is open (409), that the typing caps have room (429), and the rate
limits (429). A call refused before the rate limits is not charged.

Each channel in the channel list, and the head, carries `read` when the
caller has a read marker on it, such as `"read": {"seq": 39, "unread": 2}`.
`seq` is the marker and `unread` the number of unread messages above it,
at most 100. A channel without a marker has no `read`. A message page, a
lifecycle answer and a create answer carry no `read`. See
[Read markers and unread counts](#read-markers-and-unread-counts).

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

A message that addresses people or agents carries `mentions`, a list of
`{text, kind, ref}`. `text` is the code span as the author wrote it, so a
client can find it in the rendered markdown. `kind` is `user`, `job` or
`login`. `ref` is set for a `user` only, with the person's id, type and
title. A person the reader cannot read is left out of the list. A removed
message has no `mentions`. See [Mentions and replies](#mentions-and-replies).

```json
{ "seq": 15, "type": "note",
  "props": { "body": "`USR-7` can you check the log? `integration:triage-bot` please summarise." },
  "mentions": [
    { "text": "USR-7", "kind": "user", "ref": { "id": "USR-7", "type": "person", "title": "Ann de Vries" } },
    { "text": "integration:triage-bot", "kind": "job" } ] }
```

A channel of a type with a session carries a `session` block in the head,
the message page and the list. The `POST` of a message still answers with
the message; read the head, or follow the live frame of the notice, to see
the session move.

```json
"session": {
  "state": "contained", "label": "Contained", "status": "input_required", "generation": 1,
  "entered_at": "2026-10-21T09:12:03Z",
  "waiting_for": [ { "role": "lead", "label": "the incident lead", "holders": ["USR-1"] } ],
  "waiting_on_me": true,
  "transitions": [ { "message_type": "assessed", "to": "assessed", "guarded": true } ]
}
```

`status` is the current state's status (see
[Status and waiting_for](#status-and-waiting_for)). `waiting_for` and
`waiting_on_me` are set for an `input_required` state; `holders` and
`waiting_on_me` depend on the reader. `entered_at` is left out for a
channel that is not stored yet, so a read of such a channel does not change
with the clock. A stored channel that never moved gives its creation time.

`transitions` lists the `on:` entries of the current state, with `by:
"trigger"` when only a trigger fires one and `guarded: true` when the entry
has a `when`, and `outputs` (the number of outputs) when the entry has any.
It is the same for every reader; `_actions.transitions` says which of them
the caller fires.
`final` and `stranded` are set when they apply.

`timers` lists the timers of the current state, sorted by name, each with
its `name`, `label`, `to`, `since`, `due_at` and, when the timer has any,
`outputs` (the number of outputs). `due_at` on the session is
when the first of them comes due. Both are derived from `realtime.yaml` and
the channel. `due_at` is left out while the channel is closed or archived,
because its timers are frozen, and while the configuration is degraded,
because timers do not fire then. A final or stranded state has no timers.

`reminders` lists the reminders active in the current state, sorted by
name, each with its `name`, `label`, `every`, `count`, `max` and `due_at`.
A reminder's `due_at` is its next slot. It is left out while the channel is
closed or archived, while the configuration is degraded, and once `count`
reaches `max`. The session's own `due_at` stays the next move. A final or
stranded state, or a state outside the reminder's `in:`, has no reminders.

```json
"session": {
  "state": "detected", "label": "Detected", "generation": 0,
  "entered_at": "2026-10-21T09:12:03Z", "due_at": "2026-10-22T09:12:03Z",
  "transitions": [ { "message_type": "contained", "to": "contained" } ],
  "timers": [ { "name": "no-containment", "label": "Not contained within a day",
                "to": "overdue", "since": "state", "due_at": "2026-10-22T09:12:03Z" } ],
  "reminders": [ { "name": "status-update", "label": "Status update", "every": "30m",
                   "count": 2, "max": 48, "due_at": "2026-10-21T10:22:03Z" } ]
}
```

A notice looks like this:

```json
{ "seq": 8, "type": "session", "author": { "user": "system:realtime", "system": true },
  "triggered_by": "session:breach-response",
  "props": { "from": "detected", "to": "contained", "generation": 1, "cause": 7 } }
```

A notice of a move with outputs also carries `outputs`. A created entity
the reader cannot read shows as `{"restricted": true}`:

```json
"outputs": {
  "status": "failed", "writer": { "user": "dana", "tool": "data-entry" },
  "results": [
    {
      "kind": "create", "status": "done",
      "entity": { "id": "TL-4", "type": "timeline-entry", "title": "Contained by dana" }
    },
    { "kind": "set", "status": "failed", "code": "invalid", "props": ["risk"] }
  ]
}
```

A reminder looks like this:

```json
{ "seq": 12, "type": "reminder", "author": { "user": "system:realtime", "system": true },
  "triggered_by": "reminder:breach-response/status-update",
  "props": { "body": "Post a status update: what changed since the last one?" } }
```

### Errors

Every error is a problem body (`application/problem+json`) with a `type`.

| Status | When |
| --- | --- |
| 404 `not_found` | Realtime is off, or the entity is hidden, does not exist, or is not of the collection's type. The body is the same as for the entity `GET`. |
| 404 | The channel type is not attached to the entity type, or the channel or the message does not exist. |
| 403 | The caller may not perform the operation, the edit window has passed, the lifecycle operation is not declared (`lifecycle_not_offered`), or its `condition` does not hold (`lifecycle_denied`). For actions: the message type does not declare the action, or this process cannot promote (`action_not_offered`), the action's gate refuses (`action_denied`), or the create ACL or a `fields:` grant refuses the promote (`promote_denied`). For a post: the poster may not write an output of the transition the message would fire (`session_output_denied`); nothing is stored. `title` says what the operation needs and `detail` names the rule in `realtime.yaml`, such as `channel_types.room.message_types.text.post`. |
| 400 | The message does not match its type (`invalid_message`), has too many properties, or a removal lacks a required reason. |
| 400 | The body is not valid JSON, has unknown fields, or a query parameter is not a positive number. A mark read names a `seq` below 0 or one that is not a whole number. The inbox `kinds` names something other than `waiting`, `unread`, `mention` or `reply` (`invalid_request`). |
| 422 `too_many_mentions` | A post or edit addresses more than 10 people and agents. Nothing is stored. See [Mentions and replies](#mentions-and-replies). |
| 403 `no_principal` | The request carries no identity. A mark read and the inbox need one, because they act for a person. |
| 405 | A method other than `PUT` on `.../_read`. `Allow` names `PUT`. |
| 413 `too_large` | The message is over `message_bytes`. |
| 422 | A `ref` names an entity that does not exist or that the caller cannot read. Both get the same body. |
| 422 `promote_invalid` | The promoted entity or its relation fails validation, for example a `unique` property that another entity already has, or a mapped `ref` names an entity the caller cannot read. The body names properties, never entity ids. |
| 409 | The channel is not open, the channel is not in a state the lifecycle operation starts from (`lifecycle_wrong_state`), `retention` keeps the channel (`retention_active`), a concurrent change won (`conflict`), the message was removed, `channels_per_entity` is reached, the POST tried to open a channel of a `one` type, or the configuration changed during the request. An edit or removal targets a session notice (`session_notice`), or an author's edit or delete targets a message that moved the session (`session_message`). A post of a type whose `in:` does not hold the session's state (`session_wrong_state`; `title` names the allowed states, `detail` the rule). For actions: the message already carries an action (`action_taken`), its action is pending (`action_pending`, also for an edit, a removal, a release within the release window, or an action that lost a race to a promote that started first), a release finds no pending action (`action_not_pending`), or the promoted entity already exists (`promote_conflict`). A removal of the message that moved the session, or a release of a notice's outputs mark, while the outputs still run (`outputs_pending`). |
| 429 | A rate limit. `Retry-After` gives the seconds to wait, rounded up. |
| 400 `invalid_cursor` | The inbox `cursor` is not one an inbox answer gave, or comes with `kinds` that do not hold `waiting`. |
| 503 `inbox_too_large` | The inbox would read more than 20 000 channels. Nothing is listed; see [Inbox](#inbox). The `detail` suggests `kinds=unread,mention,reply`, which still answers. |

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

A mark read sends no frame to other readers. When it moves your marker, only
your own subscriptions get a frame: those on the channel's head and on the
entity's channel list, for example in another tab. The frame lets that tab
show the new `read`; the channel's revision stays the same, so there are no
messages to fetch. A mark that does not move the marker sends no frame.

The inbox (`/api/v1/_inbox`) and its summary (`/api/v1/_inbox/summary`)
can be subscribed to as well. [Inbox](#inbox) says when they get a frame.

A single server process delivers frames for the messages posted through it;
it does not yet poll for messages posted by another process. The inbox is
the exception: it is checked again at least every 15 seconds.

## Agents and other processes

Agents use channels through the MCP channel tools (see the MCP server
guide). Remote MCP runs inside rela-server; stdio `rela mcp` is a separate
process. Scripts and shell users can use `rela channel` (see the CLI
reference), which is a separate process too.

**Process locality.** Signals (`ephemeral` messages), rate-limit buckets and
change subscriptions live in the memory of one process. A separate process,
such as stdio `rela mcp` or `rela channel`, therefore behaves differently
from rela-server:

- It refuses a post of an `ephemeral` message type with `signals_not_kept`,
  because no one else would see the signal. Remote MCP keeps signals.
- Its rate limits count per process, not across all writers.
- A `watch` on `read_channel`, and `rela channel watch`, learn about posts
  from other processes by polling every 2 seconds. On remote MCP the poll runs every 5 seconds and
  only matters for posts made by another rela-server process; a post in the
  same process ends the wait at once.
- A post from another process reaches open browsers only when they fetch the
  channel again. rela-server does not send a live frame for it.

**SQLite.** The SQLite backend admits one process at a time. While
rela-server runs on it, stdio `rela mcp` cannot open the project, and remote
MCP is the only way for an agent to use channels, and `rela channel` cannot
run either.

**Access control.** Stdio `rela mcp` leaves its entity tools open, because
the local user can edit the files anyway. Its channel tools apply `acl.yaml`
when the project has one, as `rela lease` does, so an agent identity stays
inside the channel roles the project gives it. Locally this is not a
security boundary: a local user can edit `acl.yaml`. Without `acl.yaml`,
access is open.

`rela channel` applies `acl.yaml` the same way.

**Typing.** `read_channel` shows `actions.typing` and, with signals,
`typing_now`. There is no tool to send typing: an agent writes a message in
one step, so the indicator would show only for an instant. Over stdio
`typing_now` is always empty, because typing entries live in the server
process.

**Sessions.** `read_channel` and `list_channels` show a channel's session,
including `timers` and `due_at`, and `post_message` answers with the session
after the post. A timer's move raises the channel's rev, so a `watch` on
`read_channel` wakes on it. `rela channel read` prints a line such as
`session: contained (Contained), generation 1` under the channel header and
shows a notice as `moved from detected to contained`; `rela channel post`
prints the session line after the message. When the session has a due time,
the line ends with the timer that comes due first, such as `next:
no-containment to overdue at 2026-10-22T09:12:03Z`, and a timer's notice
reads `moved from detected to overdue (timer no-containment)`.
The session also lists `reminders`, and a reminder raises the rev, so a
`watch` wakes on it too. The session line of `rela channel` ends with each
reminder, such as `reminder status-update: 2 of 48, next at
2026-10-21T10:22:03Z`, and a reminder message is marked `(reminder)`.
The session line also gives the status and whom the session waits for,
by each role's label, such as `, input_required, waiting for the incident
lead (USR-1), waiting on you`, or `, failed (stranded)`.
A message type with `in:` is offered only in the states it lists, so
`actions.post` in `read_channel` and `You may post` in `rela channel read`
change as the session moves. A post outside them answers
`session_wrong_state` with the allowed states and the current one.

**Waiting for a session.** `read_session` returns only a channel's session,
the channel's name and state, and the message types whose post by the
agent would move the session (`transitions`). It has no rev, sequence
number or messages, so its answer changes only when the session moves or
the agent's turn changes, such as after a handoff or an edit of the lead's
property on the entity. An agent waits for its turn like this:

1. Call `read_session`. If `waiting_on_me` is true, act: post one of
   `transitions`.
2. If `status` is `completed`, `cancelled` or `failed`, stop. A stranded
   session reads `failed`.
3. Otherwise call `watch` with the target
   `{"tool":"read_session","args":{"id":…,"channel_type":…}}`, then again
   with the cursor, up to 50 seconds per call, and go back to step 1 when it
   reports a change.

A post in another process is seen within the 2-second channel poll, and an
edit of the entity wakes the watch too. A
channel type without a session answers `no_session`, decided from the
configuration, so the answer is the same for an entity that exists and one
that does not. The answer starts with the same untrusted-content `notice` as
the other channel tools: holders and labels are data.

### Read markers and unread counts

A read marker records how far you have read a channel. It is a sequence
number: every message up to it counts as read. Each reader has their own
marker per channel. The unread count is the number of messages above your
marker that count as unread, at most 100.

You have a marker on a channel once you open it in the web app, mark it
read, post in it, or create it. Only then does the channel have an unread
count for you. A channel you never touched has no `read` and is not listed
as unread in the inbox.

**Which messages count.** A message counts as unread when it is above your
marker and is one of these:

| Message | Counts as unread |
| --- | --- |
| A post or reply by someone else | Yes |
| A reminder | Yes |
| A trigger post | Yes |
| Your own post | No: your post moves your marker past it. On the file store that move comes after the post is stored, and a failure is only logged; the post then counts until your next mark |
| A session notice | No |
| A removed message | No. It stops counting as soon as it is removed |
| A signal (`persist: ephemeral`) | No. A signal is never stored |

An edit does not make a message unread again.

**When a marker moves.** These move your marker:

- You mark the channel read: the web app does this for an open channel,
  `PUT .../_read`, MCP `mark_read` and `rela channel mark-read` do it on
  request.
- You post a message. The marker moves to the last message the post wrote:
  the message itself, or the session notice that follows it.
- You create a named channel. The marker starts at 0.

Reading never moves a marker. A message page, `read_channel`,
`rela channel read` and a `watch` leave it where it is. The live feed and a
`watch` read the channel on your behalf, so a read that moved the marker
would mark messages read that you never saw. Timers, reminders and
triggers move no marker either.

A marker never moves back. A `seq` above the channel's last message is
treated as the last message. A mark does not raise the channel's revision
and writes no audit record.

**Marking read over HTTP.** `PUT .../{channel}/_read` takes an empty body,
`{}` or `{"seq": n}`, where `n` is 0 or more. Without `seq` it marks the
channel read up to its last message.

```json
PUT /api/v1/risks/RISK-1/_channels/breach-response/0k3x…/_read
{ "seq": 42 }
→ 200 { "seq": 42, "unread": 0, "last_seq": 42 }
```

The answer is the stored marker (`seq`), the unread count above it, and
the channel's last sequence number (`last_seq`). The stored marker can be
above the `seq` you sent, because a marker never moves back. A `one`
channel that holds no message yet answers zeros and stores nothing. A mark
needs read access only, so a read-only token may mark read. A channel you
cannot read gets the same answer as its head `GET`, and nothing is stored.

**Which identity is the reader.** The reader is the user and organization
that a post of yours would name as its author. The tool is not part of
it.

| Caller | Reader | Notes |
| --- | --- | --- |
| Web app, remote MCP | The user entity id (`USR-7`), or the login when the project has no `user_entity_type`, with the organization | The browser and a remote agent of one user share markers |
| Stdio `rela mcp`, `rela channel` | The login (`jeroen`), with no organization | A different reader from the same person in the web app |
| Job identity (`--run-as integration:triage-bot`) | `integration:triage-bot` | Gets markers by posting or by marking read |
| `system:realtime` | None | Its timer, reminder and trigger writes move no marker |

An agent that posts as a user moves that user's marker. Over remote MCP,
an agent's post therefore marks the channel read for the user too,
including messages from colleagues that the user never saw. Run an agent
that must not change the user's read state as a job identity
(`--run-as`), so it has its own markers.

**What rela keeps about a reader.** For each channel on which you have a
marker, rela stores your user, your organization, the marker and when it
last moved. No one else can see them: no route, tool or count shows
another reader's marker or its time. The file store keeps them in
`reads.json` in each channel's directory; the database backends keep them
in the `channel_reads` table. Read markers are per-user data, like piles,
leases and user defaults.

Markers are kept as long as their channel:

| Event | Markers |
| --- | --- |
| The anchor entity is renamed | Kept, under the new id |
| The channel is detached (see [When an entity is deleted](#when-an-entity-is-deleted)) | Kept, but out of reach |
| A detached channel comes back to its entity | Back with the channel |
| The channel is deleted or purged, or `retention` erases it | Erased |
| A deleted `one` channel is created again by a post | The new channel has no markers |
| The user entity is deleted | Kept. The reader is a string, not a reference. A new user that gets the same id takes over the read positions (see below) |
| The user entity is renamed | Not followed: the renamed user starts without markers, as a message author is not followed either |

rela has no command yet to erase the markers of one reader. To remove them,
delete or purge the channels.

**Reusing a user id passes on read positions.** Markers belong to the user
id and organization, not to a person. If you give a deleted user's id, or
their login, to someone else without erasing the old markers, the new
holder sees the old markers: on which channels they can read the previous
user had a marker, and how far that user read. They see no content they
could not already read. Avoid reusing ids and logins. Erasing a reader's
markers when the user is deleted is planned in TKT-6V8K8I.

### Mentions and replies

A message can address a person or an agent. Such a message then shows in
that principal's inbox until they read it. A principal is whoever a post
names as its author: a person, or an agent with a job identity. There are
two ways to address someone:

- A **mention** names the principal in the message.
- A **reply** addresses the author of the message it replies to.

**How to write a mention.** Put the name in a code span (backticks) in a
`markdown` property of the message. The span must hold only the name.
rela reads no other property type: a mention in a `string` or enum
property is plain text. Which form names a person depends on the project:

| Mention | Person mapping on | Person mapping off |
| --- | --- | --- |
| A person, by user entity id: `` `USR-7` `` | Mentions USR-7 | Plain code |
| A person, by login: `` `@jeroen` `` | Plain code | Mentions the login `jeroen` |
| An agent, by job identity: `` `integration:triage-bot` `` | Mentions the agent | Mentions the agent |
| A `ref` property whose value is a user entity | Mentions that person, unless the property has `notify: false` | Not applicable |

Person mapping is on when `acl.yaml` sets both `user_entity_type` and
`principal_property`. Only then does the web app know a person by their
user entity id. With `user_entity_type` alone, mapping is off and the
login form applies.

- An id span mentions someone only when it names an entity of the user
  entity type. A ticket id in a code span is not a mention and does not
  count toward the limit. The mention is stored under the entity's own id,
  so `` `usr-7` `` on a store that ignores case is stored as `USR-7`.
- A login match is exact and case-sensitive, as in `acl.yaml`. A login
  with a space, a control character or an invisible format character
  (such as a zero-width space) is not a mention.
- A code span inside a link, a code block, a span with formatting inside,
  or a span across a line break is not a mention.
- Mentioning yourself does nothing. Naming one principal twice counts
  once.
- `notify: false` on a `ref` property keeps its value from mentioning the
  person, for a field such as "reported by". It is allowed on `ref`
  properties only.

**Who may mention.** Anyone who may post the message type may mention.
There is no separate permission. A person the poster cannot read is not
mentioned, and the poster is not told so. The answer to the post is the
same as for an id that is not a person at all. Mentioning a person who
cannot read the channel stores the mention, but it shows in their inbox
only once they can read the channel.

**Limits.** A message may address at most 10 principals, counted after
the names are resolved. More are refused with 422 `too_many_mentions`,
and nothing is stored. An edit counts a mention it
keeps of a person the editor can no longer read (see below).

A principal keeps at most 200 mention and reply rows per channel. When a
new row would make 201, rela frees one slot in this order:

1. The oldest row that the principal has read under every read marker
   they hold on the channel.
2. Otherwise, the oldest unread row from the same poster as the new row.
3. Otherwise, the new row is not stored. The server logs the channel, the
   seq and the target, never the content.

rela never removes another poster's unread row to make room. So one poster
cannot erase the mentions that others sent. The cost is that a principal
with 200 unread rows from other posters in one channel gets no new rows
there until they read.

**What the poster sees.** The answer to a post or edit lists the
`mentions` the poster may see (see [HTTP API](#http-api)). It says nothing
about the target: not whether they can read the channel, nor whether they
will see the mention.

**Replies.** A reply (`reply_to`) addresses the author of the parent
message, with these exceptions:

| Parent | Reply row |
| --- | --- |
| A message by another user | Yes, for the parent's author |
| Your own message, from any tool | No |
| A message by the same user name in another organization | No: a row names a user, not an organization |
| A trigger post, a reminder, or a message by `unknown` or a rela identity | No |

A reply that also mentions the parent's author shows once in the inbox,
as a mention.

**Edits.** An edit recomputes the mentions from the new text. A mention
that stays keeps its time. A new mention gets the time of the edit. A
mention that is gone is removed. An edit keeps a mention of a person the
editor can no longer read, as long as the span or ref value is still in
the text. That mention counts toward the limit of 10. The editor wrote it
while they could see the person, so this reveals nothing new. An edit
never adds or removes a reply row.

**Read state.** The read marker decides whether a mention or reply is
unread (see [Read markers and unread counts](#read-markers-and-unread-counts)).
It leaves the inbox once the target's marker on the channel reaches its
seq: through `mark_read`, `PUT .../_read`, the web app, or the target's
own later post in the channel. A mention does not create a marker. A new
mention added by an edit to a message the target has already read does
not show. Removing the message removes its mention and reply rows.

**Retention.** Mention and reply rows live with their channel:

| Event | Rows |
| --- | --- |
| The message is edited | Recomputed, as above |
| The message is removed | Removed |
| The parent of a reply is removed | The reply row stays |
| The anchor entity is renamed | Kept, under the new id |
| The channel is detached, or comes back | Kept, out of reach while detached |
| The channel is deleted or purged | Erased |
| The person's user entity is renamed | Kept under the old id, so the person loses the open mentions. Planned in TKT-6V8K8I |
| The person's user entity is deleted | Kept, never shown to anyone else. A new user with the same id sees the author and channel of entries for channels they may read, never content |

The file store keeps the rows in `mentions.json` in each channel's
directory; the database backends keep them in the `channel_mentions`
table. A row records that one principal addressed another in a channel.
It is per-user data, like read markers.

**The scan window.** The inbox looks only at the caller's 2000 newest rows
across all channels, of the kinds the request asks for. It checks every
unread row in that window against what the caller may read before it
counts or cuts, so rows in channels the caller cannot read never push a
visible row out. An unread row older than 2000 newer rows of its kind is
not listed. The window is fixed: `mention_count` counts what the window
holds, not everything ever sent.

**Rendering trusts the stored form.** A mention is stored with its form
(`user`, `job` or `login`) when the message is posted. Turning person
mapping on or off later does not change how old messages render or whom
they address. Messages stored before this feature have no mentions.

**Which identity a mention reaches.** A mention or reply reaches one
principal key: the user that a post of that principal names as its author.
A person who uses two transports can have two keys:

| Transport | Key | A mention of this key is seen in | A reply to a message posted here goes to |
| --- | --- | --- | --- |
| Web app, remote MCP, mapping on and matched | The user entity id (`USR-7`) | The web app and remote MCP | `USR-7` |
| Web app, remote MCP, mapping off | The login (`jeroen`) | Every transport of that login | `jeroen` |
| Stdio `rela mcp`, `rela channel` | The login (`jeroen`) | Stdio and the CLI, and the web app when mapping is off. With mapping on, only a reply reaches this key | `jeroen`, so the web app does not show it when mapping is on |
| Job identity (`--run-as integration:triage-bot`) | `integration:triage-bot` | Every transport | `integration:triage-bot` |

With person mapping on, a mention of `` `USR-7` `` does not reach the
same person over stdio or the CLI. A reply to a message they posted over
stdio does not reach them in the web app. Agents use job identities,
which are the same everywhere.

A login that person mapping does not match, such as a break-glass admin
login without a user entity, cannot be mentioned while mapping is on. No
id names it, and the `@login` form is off. A reply to a message by that
login still reaches it.

**The wake.** A post or edit that addresses a principal wakes an MCP
`watch` on that principal's own `list_inbox`, when the watch asks for
`mention` or `reply`. This works only within one process. A watch in
another process, such as stdio `rela mcp` while the web app takes the
post, sees the mention on its next poll, at most 15 seconds later.

**Agents and untrusted text.** Anyone who may post can put text in front
of an agent by mentioning its job identity. An inbox entry carries no
message text. The text comes only through `read_channel`, whose answer
starts with the untrusted-content `notice`. An agent must treat the text
of a mention as data, never as instructions. Rules that limit who may
mention an agent are planned.

### Inbox

The inbox lists every session that waits on you now: each open channel
whose session is in a state with `waiting_for`, where you hold one of
those roles. It answers "what is waiting on me" without knowing the
channels in advance. It also lists the channels with messages you have not
read, and the unread messages that mention you or reply to you. The web app serves it as `GET /api/v1/_inbox`, MCP as `list_inbox`
and the CLI as `rela channel inbox`. All three give the same entries.

**Kinds.** `kinds` chooses what the inbox lists: `waiting` (the sessions
that wait on you), `unread` (the channels with unread messages), `mention`
(the unread messages that mention you) and `reply` (the unread replies to
your messages). All four is the default. The web app takes
`?kinds=waiting,unread`, MCP an array such as `["unread"]`, and the CLI
`--kinds unread`. Any other kind is refused with `invalid_request`.

An entry names the channel (anchor, entity type, anchor title, type, name,
title and state), the session (state, label, status, `waiting_for`,
generation and `entered_at`), the waiting `roles` you hold in the order of
`waiting_for`, `since` (when the session entered its state) and
`transitions`: the message types your post would move the session with, as
`read_session` lists them. The web app adds `links` with the API paths of
the entity and the channel. An entry carries no message content. Like the
other channel answers it starts with the untrusted-content `notice`:
titles, labels and holders are data.

```json
{ "notice": "…", "count": 1, "truncated": false, "degraded": false,
  "entries": [ { "kind": "waiting",
    "channel": { "anchor": "RISK-1", "entity_type": "risk", "anchor_title": "Laptop lost",
                 "type": "breach-response", "state": "open" },
    "session": { "state": "contained", "label": "Contained", "status": "input_required",
                 "waiting_for": [ { "role": "lead", "label": "the incident lead" } ],
                 "generation": 2, "entered_at": "2026-10-27T09:00:00Z" },
    "roles": ["lead"], "since": "2026-10-27T09:00:00Z", "transitions": ["handoff", "close-out"] } ] }
```

**Order and paging.** Entries are ordered by `since`, oldest first, then
by anchor id, face, channel type and name. `count` is the size of the
whole list, not of the page. `next_cursor` is present when more entries
follow; pass it back as `cursor`. A cursor only positions the list: one
from another person, or one whose entity you can no longer read, serves
nothing you cannot see.

**Unread channels.** The first page adds `unread_channels` and
`unread_count` when `kinds` includes `unread`:

```json
"unread_channels": [
  { "channel": { "anchor": "RISK-2", "entity_type": "risk", "anchor_title": "Server room",
                 "type": "breach-response", "name": "7m2c…", "title": "Breach 13", "state": "open" },
    "seq": 10, "last_seq": 14, "unread": 3,
    "links": { "entity": "/api/v1/risks/RISK-2",
               "channel": "/api/v1/risks/RISK-2/_channels/breach-response/7m2c…" } }
],
"unread_count": 1
```

Each entry names the channel, your marker (`seq`), the channel's last
sequence number (`last_seq`) and the unread count, at most 100. Only the
web app adds `links`. An entry carries no message content.

- Only channels on which you have a marker are listed, and only those with
  an unread count above 0.
- The channel with the newest activity comes first, then by channel.
  Activity is the last time the channel got a message.
- At most 50 channels are listed. `limit` does not change that.
  `unread_count` is the number of all such channels, also when it is above
  50.
- The inbox looks at no more than 500 of your markers. It takes the
  channels with messages above your marker first, then the others, and in
  each group the newest activity first.
- Later pages (with `cursor`) carry no unread fields, because a cursor
  pages the waiting sessions only.

**Mentions and replies.** The first page adds `mentions` and
`mention_count` when `kinds` includes `mention` or `reply`:

```json
"mentions": [
  { "kind": "mention",
    "channel": { "anchor": "RISK-2", "entity_type": "risk", "anchor_title": "Server room",
                 "type": "breach-response", "name": "7m2c…", "title": "Breach 13", "state": "open" },
    "seq": 15, "at": "2026-10-27T10:04:00Z", "message_type": "note",
    "author": { "user": "USR-3" },
    "links": { "entity": "/api/v1/risks/RISK-2",
               "channel": "/api/v1/risks/RISK-2/_channels/breach-response/7m2c…",
               "message": "/api/v1/risks/RISK-2/_channels/breach-response/7m2c…/messages/15" } }
],
"mention_count": 1
```

Each entry names the channel, the message's `seq`, its time (`at`), its
message type and its author. `kind` is `mention` when the message
mentions you, else `reply`. A reply carries `reply_to`, the seq of your
message. Only the web app adds `links`, with the path of the message. An
entry carries no message text: read the message in its channel.

- There is one entry per message. A reply that also mentions you is a
  `mention`.
- The newest message comes first. At most 50 are listed. `limit` does not
  change that.
- `mention_count` is the number of entries in the scan window, at most
  500, also when it is above 50. It is present, as 0, when there are
  none.
- Only unread messages are listed: those above your read marker on the
  channel.
- Later pages (with `cursor`) carry no mention fields.

[Mentions and replies](#mentions-and-replies) describes how to address
someone, the limits, and the scan window.

Without `waiting` in `kinds`, the answer has no `entries`, `count`,
`truncated` or `next_cursor`, so an absent list is not read as "nothing
waits on you". A `cursor` is then refused with `invalid_cursor`. When the
waiting half answers `inbox_too_large`, ask for
`kinds=unread,mention,reply` to still get the rest.

**What counts.** The inbox applies the same rules as `waiting_on_me` in
`read_session`. A named role counts when the role's property on the entity
holds your identity and you can read that property; a creator role counts
when you created the channel. You must be able to read the entity and the
channel. The list is gated and redacted before it is counted, paged or
cut, so `count`, `truncated` and `next_cursor` never reflect an entry you
cannot see. The same holds for unread channels: a channel you can no
longer read is not listed and does not change `unread_count`. It holds for
mentions and replies too: a message in a channel you cannot read, on an
anchor or face you cannot read, or of a channel type the configuration no
longer declares is not listed and does not change `mention_count`.

**Caps.** When you hold a waiting role on more than 20 000 entities, the
inbox keeps the first 20 000 by id and face and sets `truncated`. When the
inbox would read more than 20 000 channels, it lists nothing and answers
`inbox_too_large` (503 on the web app, a tool error on MCP and the CLI).
A degraded configuration answers an empty list with `degraded: true`, and
no unread channels or mentions.

**Which identity holds a role.** A named role compares the property with
the identity the caller acts as:

| Caller | Named role (`from: entity.<prop>`) | Creator role |
| --- | --- | --- |
| Web app, remote MCP | The property holds the user entity id (`USR-7`), or the login when the project has no `user_entity_type` | Channels opened in the web app or over remote MCP |
| Stdio `rela mcp`, `rela channel` as a person | The property holds the login (`jeroen`) | Channels opened over the CLI or stdio |
| Job identity (`--run-as integration:triage-bot`) | A string or list property holds `integration:triage-bot`. A `ref` property to the user entity type cannot hold it | Channels the job opened |
| Any client on a read-only token | Never: a waiting role is held for update, which the token does not grant, so the inbox is always empty | Never, for the same reason |

Remote MCP cannot carry a job identity, so a job agent uses stdio. Stdio
cannot open a SQLite project while rela-server runs on it, so run job
agents against the file or PostgreSQL backend.

**The agent loop.**

1. Call `list_inbox`.
2. For each entry, read it with `read_session` or `read_channel`, and post
   one of its `transitions`.
3. For each unread channel, read the new messages with `read_channel`
   (`after` its `seq`), and call `mark_read` when it has handled them.
4. For each of `mentions`, read the channel with `read_channel` after
   your marker, act, and call `mark_read`.
5. Call `watch` with the target `{"tool":"list_inbox","args":{}}`, then
   again with the returned cursor, up to 50 seconds per call. Go back to
   step 1 when it reports a change.

An entry leaves the inbox when the session moves, the role moves to
someone else, the channel closes, or you lose access. An unread channel,
a mention and a reply leave it when you mark the channel read or post in
it. A post in the same process
and an edit of an entity wake the watch at once. A post in another process
is seen by the poll, which replays the inbox at most every 15 seconds.

A plain message wakes the watch only when its channel is among the
channels the inbox looked at for unread messages (see Unread channels). A
message in any other channel does not wake it, even one of the same
channel type. A channel you mark read for the first time after the last
replay is picked up at the next replay or poll. With `"kinds":
["waiting"]` in the target's args, no plain message wakes the watch.

A post or edit in the same process that mentions you, or replies to you,
wakes the watch, also in a channel the inbox did not look at. It wakes
only a watch bound to your own principal key, and only while the target's
`kinds` hold `mention` or `reply`. A mention of someone else does not
wake it.

**Summary.** `GET /api/v1/_inbox/summary` answers the counts of the
inbox as the web app's sidebar badge shows them:

```json
{ "waiting": 3, "waiting_truncated": false, "unread_channels": 2, "mentions": 1, "degraded": false }
```

`waiting`, `waiting_truncated`, `unread_channels`, `mentions` and
`degraded` are `count`, `truncated`, `unread_count`, `mention_count` and
`degraded` of `GET /api/v1/_inbox` with its default parameters. The query
string is ignored. The summary refuses like the inbox: 404 with realtime
off, 403 `no_principal` without an identity, and 503 `inbox_too_large`.

**Live updates in the web app.** A client can subscribe to
`/api/v1/_inbox` and `/api/v1/_inbox/summary` on the live event stream.
The inbox path takes `limit` and `kinds`, each at most once. It does not
take `cursor`: only the first page is live. The summary path takes no
query. Any other key, a repeated key, or a bad value is refused with 400.
Equal requests are the same subscription: `kinds=mention,waiting` and
`kinds=waiting,mention,waiting` are one, and so are `limit=05` and
`limit=5`.

Like every live path, the server replays the GET as you and sends a frame
only when the answer changed. It replays your inbox when one of these
happens in the server process:

- a stored change in a channel of a type that has a waiting state: a
  message, an edit, a removal, a session or lifecycle move, or an action;
- a stored change in a channel the inbox looked at: one where you have a
  read marker (see Unread channels), or one with an entry under
  `mentions`;
- a move of your own read marker, by a mark read or by your own post;
- a post or edit that mentions you or replies to you, while the
  subscription's `kinds` hold `mention` or `reply`;
- an edit of an entity.

Typing and `ephemeral` signals, and their expiry, do not replay an inbox.

An inbox is replayed on a fixed clock that is the same for every
subscription: at most every 2 seconds when channels are stored in
PostgreSQL or SQLite, and at most every 4 seconds with the file channel
store. The file store reads a person's read markers and mentions by
scanning every channel, so one round of inbox replays takes longer; the
clock step is chosen so that one round finishes within it. The steps
are counted in wall-clock time, so a slow round does not move later
steps. A change waits for the next step of that clock. So when an inbox
round starts does not depend on changes you cannot see. How long the
round takes can still vary, as it does for every full round: frames
leave when the whole round has finished, and the round is longer when
more inboxes need a replay, including for changes you cannot see. An
inbox that does not fit in a round (the round's limit on replays or on
time) waits for the next step and gets no frame until its answer
changes.

The server also replays every inbox subscription once every 14 seconds
(12 seconds with the file store), whether or not anything changed. The
people are spread over the steps of that period, so each step replays
only a share of them. That picks up a post made in another process, such
as `rela channel post` or stdio `rela mcp`, within 15 seconds.

An inbox whose last replay was refused, for example because realtime was
off, is replayed again only after an entity edit or when realtime is
turned on again.

The summary and the first page of one person read the same inbox: the
server computes it once per person per replay round.

**Outputs.** `post_message` answers `outputs` for a post whose move wrote
to the graph, and notices in `read_channel` carry the same block (see
[Outputs](#outputs)). A post the agent may not complete is refused with
`session_output_denied` before it is stored. Each step of the outputs
raises the channel's rev, so a `watch` wakes on it. `rela channel read`
adds the outputs to the notice line, such as `moved from assessed to
notified; outputs done: set DL-1` or `outputs failed: set DL-1 (invalid:
reference)`, and `rela channel post` prints the same outputs line after the
session line.

**Identity.** The channel tools and `rela channel` act as `$USER`, or as the
job identity in `RELA_RUN_AS` (`--run-as`), such as `integration:triage-bot`.
Give an agent its own job identity and an `acl.yaml` assignment, so it holds
only the channel roles it needs. Messages show the job identity as their
author. The OS user who asserted the identity is recorded only in graph
audit records, as `asserted_by`. Channel posts, edits, removals and channel
creation write no graph audit record, so for them nothing records the OS
user: the channel and its audit trail name the job identity only. The name is
self-asserted: it is attribution, not authentication. A job identity also
has its own read markers, so its posts do not mark channels read for a
person.

When `acl.yaml` exists and its `assignments` give that identity no role, and no user entity or membership
gives it one, the channel tools can answer `entity not found` for an entity that
`show_entity` returns, and `rela mcp` logs a warning at startup. Add the
identity under `assignments`. `rela mcp` reloads `acl.yaml` when it changes.
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
  event naming `realtime.yaml`. After a reload that succeeds, open browsers
  get a `refresh` event, so they read `GET /api/v1/_schema/channels` again.
- After a `realtime.yaml` reload, rela-server reads `data-entry.yaml` from
  disk again and checks its `channels:` block against the new file (see
  the data-entry guide). The two reloads run one at a time. If
  `data-entry.yaml` loads, it is put in force with the new `realtime.yaml`.
  If it does not, the new `realtime.yaml` stays in force, the previous
  `data-entry.yaml` keeps working, and open browsers get a `config-error`
  event naming `data-entry.yaml`, with `"applied": "realtime.yaml"`.
- A change to `anchor:` of a channel type that has stored channels is
  refused, on reload and when the server starts, because the stored
  channels are keyed by the old anchor. Migrating channels to another
  anchor is not supported yet.
- A reload may turn `typing` on or off. Turning it off hides the live
  entries at once and refuses new typing calls with `typing_not_offered`.
- A reload may change a session freely: add or remove states, rename
  `initial`, or drop the `session:` block. A session that never moved
  follows the new `initial`. A channel in a state the new file drops is
  stranded (see Sessions). Without the `session:` block the session is
  hidden from reads; the stored state stays and returns with the block.
  rela-server logs one warning per channel type and stranded state after
  each reload, with the number of sessions in it.
- A reload may add, remove or change timers. A timer counts from its
  anchor, so adding or shortening one can fire it within a minute, for
  sessions old and new. Removing a timer, or the whole `after:`, clears the
  due time. Reads show the new due time at once.
- A reload may add, remove or change reminders. A changed `every` or
  `first` takes effect from the last reminder: the next one comes at the
  first new slot after it, so at most one `every` later. Adding a reminder
  to a state of an old session posts once at the next check, then follows
  the slots. Removing a reminder stops it and drops its count at the
  channel's next write. Removing the current state from a reminder's `in:`
  stops it there, and its count reads 0.
- A schema change can make a reminder's `props` fail the checks of a
  post, for example when an enum loses the value it posts. rela then
  logs this once and pauses the reminder for every channel until
  `realtime.yaml` or the schema changes again. Timers keep firing.
- A reload may add, remove or change outputs and `on_final`. The check
  before a post uses the file in force then. A retry reads the outputs
  from the current file: changed outputs run as now declared, and a
  transition the file no longer declares fails `transition_removed`. A
  timer's or trigger's mark whose `run_as` the file changed fails
  `run_as_changed`. While the configuration is degraded, a post that would fire outputs is
  refused with 403 `session_output_denied`, and retries wait until a valid
  configuration is back.
- Timers and reminders do not run while the configuration is degraded. They
  run on the first check after a valid configuration is back.
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
