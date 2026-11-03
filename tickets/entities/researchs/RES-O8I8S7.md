---
id: RES-O8I8S7
type: research
title: 'rela realtime phase 4: sessions as channel types with a statechart'
summary: 'Sessions extend channel types: flat statechart with session timers and count() guard; state snapshot on the channel row with CAS; next_due_at timers fired by the realtime worker leader; per-type run_as for timer writes; roles from graph properties; seven slices.'
status: done
---

# Draft research: rela realtime phase 4, sessions

Status: draft for user review. Builds on RES-1UCUBA (sub-decision C3, the
channels and channel lifecycle follow-up decisions, the multi-instance seam).
Code citations are against `origin/develop` (d4ed867c4, 2026-10-21).

## Problem

Phase 3 gave rela typed channels: conversations anchored to an entity, with
typed messages, roles, actions, `promote_to`, a lifecycle and entity-change
triggers. A channel has no notion of *where the conversation is*. Nothing in the
config can say "this incident room is in `contained`; the risk assessment is due
in 4 hours; only the incident lead may declare it assessed; if the authority is
not notified within 72 hours, escalate".

RES-1UCUBA decided that sessions are channel types with a declarative statechart
(C3, follow-up decision 7). Typed messages are the events, timers are durable,
and outputs reach the graph through `promote_to` and actions. This document
turns that decision into a scope, a set of design choices and a slice plan.

The questions:

1. How much statechart language is needed, and what is deliberately left out?
2. Where does session state live, and how does it survive restarts, config
reloads and several server processes?
3. How do timers work durably on the fs, sqlite and postgres tiers?
4. What triggers a transition, who may trigger it, and how do outputs reach
the graph when no person is present (a timer fired)?
5. What is the smallest UI and agent surface that makes sessions usable?

## Context

### What phase 3 built

| Area | What exists | Where |
| --- | --- | --- |
| Config | `realtime.yaml`: `limits`, `channel_types`, `triggers`. A channel type has `attach_to`, `anchor`, `cardinality`, `create`, `read`, `roles`, `message_types`, `lifecycle`, `message_ops`, `on_entity_delete`. `session` is a **reserved, refused key** ("sessions are not supported yet"). | `internal/realtimecfg/raw.go`, `keys.go:18-19`, `docs/realtime.md` |
| Roles | Derived, admit-only. Sources: `permission:`, `from: attached`, `from: entity.<prop>`, `principal_type:`, `creator:`. Every role also needs read of the anchor. No role is *assigned* at runtime. | `internal/realtimecfg/compiled.go:200-240` |
| Agents | Deliberately ordinary principals. `agents:` and `actions.*.by` are refused keys: limit agents with `client_baselines` in `acl.yaml` and match them with a `principal_type` role. | `internal/realtimecfg/keys.go:20-23` |
| Message types | `persist` (`log` or `ephemeral`), typed `properties` (metamodel types, `ref` with `to:`), `when:` predicate on the message, `post: [roles]`, `rate`, `actions` with `promote_to {type, face, map, relate}`. | `raw.go`, `internal/realtime/validate.go` |
| Actions and promote | An action is a CAS state on the message (`none -> pending -> done`, release by a moderator). The `Promoter` creates the entity **as the caller**, through the entity manager, so ACL, validation, automations and graph audit apply; `triggered_by` is `channel:<key>#<seq>/<action>`. pg/sqlite commit entity, relation and action mark in one transaction. | `internal/realtime/promote.go`, `actions.go`, `internal/appbuild/realtime_promote*.go` |
| Lifecycle | `open`, `closed`, `archived`, delete. Operations declared per type with `roles`/`permission`, `condition`, `from`, `denied`. `auto.when` (entity predicate, edge-triggered) and `auto.after` (period since `StateSince`). Transitions go through `Store.SetLifecycle` (CAS on state), post a system message and write a channel audit record with `triggered_by: auto:<type>/<op>`. | `internal/realtime/lifecycle.go`, `sweep.go`, `internal/channels/store.go` |
| Timers today | **No timer service.** `auto.after` is derived: the sweep lists channels per (type, state) that entered the state before a cutoff (`Store.ListByLifecycle`) every minute. The pending-action and tombstone purges use the same polling style (`StalePending`, `DueTombstones`). | `internal/realtime/sweep.go:150-205`, `internal/appbuild/realtime_worker.go` |
| Background worker | One realtime worker per process; one **leader** across processes (pg session lock, file lock on fs). Leader runs the sweep each minute, retention each hour, and the trigger engine (`TriggerTick` 500ms rounds, 15 min reconcile). Two leaders can overlap for up to 10s, so trigger posts are at least once. | `internal/appbuild/realtime_worker.go:24-50` |
| Triggers | Entity-change engine: `enter`/`leave`/`change` on a predicate, posts a literal system message as `system:realtime`. Seen-sets in `state.KV` so a new leader carries on. Cooldown 1/min per (rule, entity), capped posts per round, retries for 5 min. | `internal/realtime/trigger.go`, `trigger_rules.go` |
| Channel store | `channels.Store`: gapless `seq`, `rev`, CAS on lifecycle and actions, channel audit trail, tombstones, retention. Backends: `filechannels` (directory per channel, JSONL segments, file lock), `sqlitechannels` and `pgchannels` over shared `sqlchannels`. Conformance in `channelstest.RunAll`. | `internal/channels/` |
| Agent surface | MCP tools `list_channels`, `read_channel` (with `watch`), `post_message`, `edit_message`, `remove_message`, `create_channel`, plus a generic `watch` tool. CLI `rela channel` with `--run-as`. One JSON schema per message type. | `internal/mcp/tools_channels*.go`, `tools_watch.go`, `docs/realtime.md` "Agents" |
| Browser surface | Channels panel on the entity page, live via SSE subscriptions on the channel list and head paths; composer per message type; action and lifecycle buttons shown only when allowed. `data-entry.yaml` `channels:` block for field order, labels, widgets. | `frontend/src/composables/useChannel.ts`, `useEntityChannels.ts`, `frontend/packages/rela-components/src/components/channel/`, `internal/dataentryconfig/channels.go` |
| Mockup | Storybook "Mockups/Realtime" already sketches a session header: state stepper, timers with `interrupting` vs `reminder`, a system message per transition. | working tree: `frontend/packages/rela-components/src/fixtures/RlRealtimeSession.vue`, `realtime.ts:98-103,406-424` |

### Pieces to reuse

- **`internal/predicate`**: concurrency-safe, no I/O. Already compiled for
message `when:` and lifecycle `condition:`. Transition guards use it with a
record that holds `message`, `entity` (allow-all header read, as triggers do)
and `session` (state, entered time, counters).
- **`internal/statemachine`**: compiles enum `transitions:` (edges, guard
permission, `when:`). It is entity-keyed and write-path shaped
(`EnforceUpdate(old, new)`), so it is not reusable as an engine. Its
*conventions* are worth copying: compile once at load, index edges, sentinel
errors for illegal/guard/precondition, load-time well-formedness.
- **Channel store CAS**: `SetLifecycle(expect)` and `SetAction(expect)` are
the model for a `SetSession(expect state, expect generation)` step.
- **Realtime worker and leader lock**: the natural host for a timer loop.
- **Trigger engine**: already turns entity changes into typed system messages.
If messages drive transitions, entity-driven transitions come for free by
composition (trigger posts a message, the message moves the session).
- **Pending-action pattern**: `pending -> done` with release covers graph
outputs whose write might fail half way.
- **`lease`**: CAS, TTL, fencing token, `NextExpiry` and a wake `Hub`
(`internal/lease/wake.go`). Leases are keyed by entity id, not by role, so they
do not fit role holders directly; the `NextExpiry` plus wake idea fits the timer
loop.
- **`jobs.Queue`**: no delayed or run-at jobs (`internal/jobs/jobs.go`), and
`WithDeferral` is built but unwired. Sessions should not need it except for
external side effects (mail on escalation), which go on the queue as usual.
- **Scheduler `run_as`**: an identity that is granted rights in `acl.yaml`,
never inferred (DEC-O59WM4, `internal/scheduler/config.go:37-50`,
`principal.ValidateRunAs`). The precedent for unattended graph writes.

### Use cases, prioritized

| # | Use case | Statechart features needed |
| --- | --- | --- |
| 1 | **Incident / data breach response** (atlas `datalek`, GDPR 72h) | States with an order; role-guarded transitions (lead declares `contained`); interrupting deadline (72h to `notified` or escalate); non-interrupting repeating reminder ("status update every 30 min") active across several states; outputs (timeline entry, set entity status); auto-close the channel at the final state. |
| 2 | **Agent asks a human (`needs-human`, `input_required`)** | A waiting state that names whom it waits for; human-only answer (a role only humans hold); timeout that hands back or escalates; MCP agent can wait on it. |
| 3 | **Approval with quorum and deadline** (decision vote) | Counting votes from distinct authors; guard on the count; deadline that rejects or escalates; output writes the decision. |
| 4 | **Handoff** (agent to human, shift handover) | Changing who holds a role, recorded durably; acknowledge step with timeout. |
| 5 | **Review rounds** (pair review with agent reviewer) | Loop between `reviewing` and `revising`; round counter; outputs create review-response entities (already possible with `promote_to`). |
| 6 | Standup / meeting turns | Turn policy (round robin over present participants). Lowest priority: rare, and the value is mostly UI. |

Use cases 1 and 2 drive the first slices. Quorum (3) needs a small counting
feature. Turns (6) can wait.

### Prior art, only what informs the design

| System | Lesson |
| --- | --- |
| **XState / SCXML** | `after:` delayed transitions are cancelled on state exit; this "timer belongs to the state" rule makes stale timers impossible by construction. Hierarchy and parallel regions exist mostly to share transitions and timers across states; a session-level timer list with `in: [states]` covers rela's cases without them. `context` holds counters (quorum). |
| **AWS Step Functions (ASL)** | Flat states, `TimeoutSeconds`, `HeartbeatSeconds`, `Catch: States.Timeout`. A flat machine with per-state timeouts carries real approval workflows. `waitForTaskToken` is the shape of `input_required`. |
| **Temporal** | Timers are durable rows in the workflow history, fired by a timer queue; a fired timer is just an event the workflow handles. Signals (fire and forget) differ from updates (validated). In rela: a typed message with `when:` is a validated update. |
| **BPMN boundary timers** | Two kinds: interrupting (leave the state) and non-interrupting (fire a side path, stay). Cycles (`R/PT30M`) for reminders. Matches the RES-1UCUBA wording directly. |
| **MCP tasks (2026-07-28)** | Statuses `working`, `input_required`, `completed`, `cancelled`, `failed`. A session state can carry one of these as its `status`, so an agent sees a standard value. |
| **A2A 1.0** | Task states add `submitted`, `rejected`, `auth-required`. The context (conversation) is separate from the task; in rela the channel is the context and the session state is the task status. |
| **Jira SM SLAs** | SLAs start, pause and stop on conditions. Pausing is the one feature that makes derived timers hard; leave it out of v1. |

## Options

### (a) Statechart language scope

| Option | Description | Assessment |
| --- | --- | --- |
| a1 Flat states | States, initial, final; transitions `on:` a message type with `roles`, `when:`, `to:`; per-state `after:` timers. | Covers use cases 2, 4, 5. Use case 1 needs the same reminder in four states, copied four times. |
| **a2 Flat states plus session-level timers and context counters** | a1, plus a `timers:` list on the session with `in: [states]` (or `from`/`until` states), and a small `count:` facility for guards (distinct authors of a message type, optionally filtered). | Covers 1 to 5. Keeps one state per session, so the stored state is one string. **Recommended.** |
| a3 Hierarchical and parallel (full XState) | Nested states, parallel regions, history. | Expressive, but the stored state becomes a configuration tree, the UI stepper stops being a line, and config validation grows a lot. No use case needs it. |

Proposed shape (illustrative, not final):

```yaml
channel_types:
  incident-room:
    cardinality: one
    roles: { lead: { from: entity.owner }, member: { from: attached }, dpo: { permission: privacy.officer } }
    message_types:
      contained: { properties: { note: { type: markdown } }, post: [lead] }
      assessed:  { properties: { risk: { type: risk-level, required: true } }, post: [lead, dpo] }
      notified:  { properties: { reference: { type: string, required: true } }, post: [dpo] }
    session:
      label: GDPR breach response
      initial: detected
      states:
        detected:  { label: Detected,      on: { contained: contained } }
        contained: { label: Contained,     on: { assessed: { to: assessed, when: "message.risk != 'none'" } } }
        assessed:  { label: Risk assessed, on: { notified: notified },
                     after: { 72h: { to: escalated, from: session.started } } }
        escalated: { label: Escalated,     status: input_required, waiting_for: [dpo], on: { notified: notified } }
        notified:  { label: AP notified,   final: true,
                     outputs: [ { set: { status: notified } } ] }
      timers:
        status-update: { every: 30m, in: [detected, contained, assessed], post: { type: reminder } }
      on_final: close
```

### (b) Where session state lives

| Option | Description | Assessment |
| --- | --- | --- |
| **b1 Snapshot on the channel row, transitions in the log** | The channel row gains `session_state`, `session_since`, `session_gen` (bumped per transition), `session_started`, `next_due_at`, a small counters map and the config hash. Every transition appends a system message (`type: session.transition`, from, to, cause) and a channel audit record, as lifecycle does. | One read gives the state; CAS on `(state, gen)`; the log already is the visible history. Matches lifecycle decision 8. **Recommended.** |
| b2 Event-sourced, replay the log | State is a fold over messages. | Replay breaks when the config changes (a renamed state, a new guard) and costs a full log read per decision. Messages are lenient history (schema hash), not a reliable event source. |
| b3 Session as an entity | C1 from RES-1UCUBA. | Rejected again: per-transition graph writes, audit noise; the anchor entity already is the durable record. |

Config change policy under b1: a stored state that the new config no longer
declares puts the session in a `stranded` condition (shown, no transitions, no
timers), and the reload logs it. Same idea as the refused `anchor:` change.

Relation to the lifecycle: the session runs only while the channel is `open`.
`closed` freezes it (timers stop, as decision 2 already says for timers). A
final state may close the channel (`on_final: close`). Reopen resumes in the
stored state; timers that came due while closed fire once on reopen, or are
skipped (open question).

### (c) Durable timer service

| Option | Description | Assessment |
| --- | --- | --- |
| c1 Derived scan (like `auto.after`) | No stored timers. Each tick, per (type, state, timer), list sessions that entered the state before `now - after`. | No new storage. Cost is one query per timer per tick; minute granularity; repeating reminders need a "last fired" counter anyway; absolute deadlines (72h from session start, not from state entry) need their own index. |
| **c2 Next-due on the row** | On every transition or timer fire, compute the earliest due instant of all timers active in the new state and store it as `next_due_at` (plus a reminder counter). One index `DueSessions(now)`; the leader loop sleeps until the nearest due time (or a 1s floor, 60s ceiling) and is woken by a local hub when a transition shortens it. A fire is a CAS on `(state, gen)`: a timer for a state the session has left fails the CAS and does nothing. Config reload recomputes `next_due_at` for the affected types. | One column, one index per backend, exact-once transitions, second precision. **Recommended.** |
| c3 Separate timer table | Rows `(channel key, timer, gen, due_at)` armed on entry, cancelled on exit. | The most general (pause, many timers per session, entity-relative deadlines), but two writes to keep consistent per transition and a cancel path that can leak. Add later only if pause or entity-relative deadlines appear. |

Tiers: `filechannels` stores the fields in `meta.yaml`; the due index is a scan
of `meta.yaml` files at start plus an in-memory heap kept by the leader
(fs/desktop are effectively single leader already). sqlite and pg get a column
and a partial index on `next_due_at`. Multi-instance: only the leader fires,
using the existing worker lock; the CAS makes a doubled fire during leader
overlap harmless for the transition. Outputs of a fire are at least once, keyed
by `session:<key>/<gen>` so a repeat is detectable.

### (d) What triggers a transition

| Option | Description | Assessment |
| --- | --- | --- |
| **d1 Typed messages and timers only** | `on: <message type>` and `after`/timers. Entity-driven moves go through an existing trigger that posts a typed system message into the session channel. | One event model. Entity changes reuse the trigger engine with its cooldown, reconcile and at-least-once rules. Message `post` roles are the transition permission, so no second permission model. **Recommended.** |
| d2 Also `on_entity:` predicates in the statechart | Transitions guarded directly by entity conditions. | Duplicates the trigger engine's edge detection inside the session runtime. |
| d3 Also explicit "transition" buttons without a message | A UI action that moves the state. | A transition without a message has no payload and no visible record; a message type with no properties already is that button. |

Who may trigger: the message type's `post` roles, optionally narrowed per
transition with `roles:`. Human-only answers follow the phase 3 rule (agents are
ordinary principals): the operator gives the answering role a `permission:` that
`acl.yaml` grants only to humans, or narrows agent clients with
`client_baselines`. No `by: human` key.

### (e) Roles and turn policies

| Option | Description | Assessment |
| --- | --- | --- |
| **e1 Durable roles from the graph** | The incident lead is `from: entity.owner` (exists today). A handoff is a transition whose output patches that property through `PatchEntity`, so the role change is in the graph audit. | Matches RES-1UCUBA ("role transfers are durable"). No new role store. **Recommended for named roles.** |
| e2 Session-scoped holders in the channel row | A `holder:` role source filled by a `claim`/`handoff` message, audited in the channel trail. | Needed for roles that are not entity properties (the "current speaker", the agent working the session). Small, but a second assignment mechanism. Add with handoff slice if e1 is not enough. |
| e3 Turn policies (round robin, moderator) | A policy over present participants. | Defer. Only use case 6 needs it. |

### (f) Graph outputs

| Option | Description | Assessment |
| --- | --- | --- |
| f1 Reuse `promote_to` and actions only | A transition message carries an action that promotes. | Works for message-driven creates. Cannot set a property on the anchor, and nothing writes when a timer fires. |
| **f2 Transition `outputs:`** | Two output kinds: `set:` (patch anchor properties through `PatchEntity`) and `create:` (same shape as `promote_to`). Message-driven transitions write **as the poster** (their authority, their ACL). Timer-driven transitions write as the session type's `run_as` identity, granted in `acl.yaml` like a scheduler task. `triggered_by: session:<key>/<from>-><to>#<gen>`. The transition is recorded first as pending, then done, reusing the action states, so a failed write is visible and releasable. | One audit story, no inferred allow-all. **Recommended.** |
| f3 Always write as `system:realtime` with allow-all | Simple. | Lets any poster launder writes they could not make; contradicts "allow-all is never inferred from identity". |

Writes happen outside channel store transactions on fs; on pg/sqlite the
existing promote path already commits entity, relation and mark together and can
do the same for the transition mark. External side effects (mail on escalation)
go on `jobs.Queue`.

### (g) UI surface, minimum

- Session header in the channel view: label, state stepper (states in
declaration order, current highlighted), active timers with remaining time and
kind, and "waiting for: <role>" when the state has `waiting_for`. The mockup in
`RlRealtimeSession.vue` is the starting point.
- Transitions are offered through the existing composer: the message types
that move the session from the current state are listed first. No new button
type.
- Transition and timer events render as system messages (exists for
lifecycle).
- `data-entry.yaml` `channels:` menu filter gains `session_state:` and
`waiting_for_me: true` (later slice).
- MCP: `read_channel` returns a `session` block (state, status, since,
waiting_for, offered message types, next due); `watch` wakes on a session
change. `status` maps to MCP tasks values.

### (h) Slicing

Each slice ends shippable and documented in `docs/realtime.md`.

1. **Session config and message-driven transitions.** `session:` block (states,
initial, final, `on:` with `to`, `roles`, `when`), load-time validation, state
fields on the channel row in all three backends plus conformance tests, CAS
step, transition system message and audit record, state in the HTTP and MCP
channel read. No timers, no outputs.
2. **Session UI.** Header with stepper, offered message types first, system
message rendering, schema endpoint exposure. Storybook from the mockup.
3. **Interrupting timers.** `after:` per state (from state entry or session
start), `next_due_at`, leader timer loop with wake, reload recompute, `stranded`
handling, freeze while closed.
4. **Non-interrupting timers.** Session-level `timers:` with `in:`, `every:`
reminders that post a typed system message, reminder counter.
5. **Graph outputs.** `outputs:` with `set:` and `create:`, poster versus
`run_as` identity, pending/done marks, `on_final: close`.
6. **Input required and handoff.** `status:` per state (MCP tasks values),
`waiting_for:`, MCP `watch` on sessions, handoff via `set:` on a role property;
session-scoped holders only if needed.
7. **Counting guards.** `count(type, distinct authors, filter)` in the guard
record for quorum votes; menu filter `session_state:`.
8. (Later) Turn policies; entity-relative deadlines and SLA pause (would move
timers to c3).

Entity-driven transitions need no slice: they are an existing trigger posting
into the session channel, documented in slice 1 or 3.

## Recommendation

Build sessions as an extension of the channel type, not a new subsystem:

- Language **a2**: flat states, session-level timers with `in:`, a small
counting facility. No hierarchy or parallel regions.
- State **b1**: snapshot on the channel row with a generation counter, every
transition visible as a system message and audited in the channel trail.
- Timers **c2**: `next_due_at` on the row, fired by the existing realtime
worker leader, with CAS on `(state, gen)` so stale or doubled fires do nothing.
Keep c3 as the upgrade path.
- Triggers **d1**: typed messages and timers only; entity changes arrive via
existing triggers.
- Roles **e1** first; e2 only if handoff needs a non-entity holder; e3
deferred.
- Outputs **f2**: as the poster for message-driven transitions, as a declared
`run_as` identity for timer-driven ones, with pending/done marks.
- UI **(g)** minimum as listed; slices **1 to 7** as in (h).

Risks:

- The trigger engine is already 1400 lines; the timer loop must stay a
separate, small component that shares only the leader lock and the store.
- `filechannels` needs a due index without a database; the leader heap plus a
start-up scan is fine for fs and desktop scale but must have a budget test.
- Timer-driven writes under `run_as` add an identity operators must grant in
`acl.yaml`; a missing grant must fail loudly at load (like the MCP warning for
unassigned identities), not silently at 3 a.m.

## Open questions for the user

1. **Language scope.** (A) flat states only; (B) flat states plus
session-level timers and counting guards; (C) hierarchical and parallel states.
Recommendation: B.
2. **Session and channel lifecycle.** (A) the session is separate and only
runs while the channel is open, with optional `on_final: close`; (B) for session
types, the session replaces the lifecycle (final state means closed); (C) the
lifecycle's `auto.when` may read the session state. Recommendation: A.
Sub-question: timers that came due while the channel was closed, fire once on
reopen or skip? Recommendation: skip and restart from the reopen, like
`auto.after`.
3. **Timer storage.** (A) derived scan like `auto.after`; (B) `next_due_at`
on the channel row; (C) separate timer table. Recommendation: B, with C as the
upgrade path if SLA pause or entity-relative deadlines are needed.
4. **Identity for timer-driven graph writes.** (A) a per-session-type
`run_as` identity granted in `acl.yaml`; (B) `system:realtime` with allow-all;
(C) timers never write to the graph, they only post messages and move state.
Recommendation: A. C is a valid smaller first step.
5. **Role assignment and handoff.** (A) graph property only (`from:
entity.<prop>`, handoff patches it); (B) session-scoped holders in the channel
store; (C) both. Recommendation: A first, B when a concrete non-entity role
appears.
6. **Quorum and counters.** (A) a built-in `count()` guard over messages in
the session; (B) general session context variables set by transitions; (C) leave
quorum out of phase 4. Recommendation: A.
7. **Cardinality.** (A) sessions only on `cardinality: one` types; (B) also
on `many` (several concurrent approvals on one entity). Recommendation: B, it
costs nothing extra under b1, but confirm the UI for several running sessions on
one entity.

## Decisions (2026-10-21)

The user chose:

1. Language: flat states, session-level timers with `in:`, and a `count()` guard (a2). No nested or parallel states.
2. Lifecycle: separate. A session runs only while its channel is open; `on_final: close` may close it. Timers due while closed are skipped and restart from the reopen.
3. Timer-driven graph writes use a per-session-type `run_as` principal granted in `acl.yaml`; a missing grant fails at load.
4. Roles come from a graph property first (`from: entity.<prop>`); handoff is an audited entity write. Per-session holders only if a later need appears.

Taken as recommended without asking: timers stored as `next_due_at` on the
channel row (c2, with a timer table as the upgrade path); sessions allowed on
`cardinality: many` channel types.
