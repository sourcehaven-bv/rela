---
id: RES-1UCUBA
type: research
title: 'rela realtime: a schema-driven interaction layer for humans and agents (presence, leases, signals, sessions, live views, chat)'
summary: 'Layered design: live views via poke + gated re-read; leases/timers in a CAS store; channels anchored to entities (outside the graph) with typed message schemas, channel-level read and per-type post permissions; signals and sessions fold into channel types; multi-instance seams designed in.'
status: done
---

## Problem

rela declares the shape of **state**: entity types, relations, validations,
transitions. It has no declarative way to express **interaction over time**: who
is present, who is working on what, what signals exist, how a multi-party
exchange proceeds, and how humans and agents talk about an entity while it
changes.

Two drivers, weighted equally:

1. **Human collaboration.** Live views, presence, chat about an entity,
co-editing a form, approvals with deadlines, incident war rooms.
2. **Agent coordination.** Several agents (and humans) working one project.
Today agents collide on the same ticket, cannot ask a human mid-task, and cannot
be told that something changed. MCP clients get no push at all.

The question: what should "rela realtime" be, which primitives does it need,
where is it declared, and how does it fit the existing ACL, store and write-path
rules? Multi-instance deployment must be possible by design; it does not have to
ship in the first phase.

## Context

### What exists today (codebase survey, 2026-10-11)

| Seam | State | Reference |
| --- | --- | --- |
| Store change feed | `store.Watcher.Subscribe` emits `store.Event{Op, EntityType, EntityID, RelationType, From, To, Face}`. Identity only, no payload, **no sequence number**, lossy (drops on full buffer). pg/sqlite emit post-commit; fs/mem under the store mutex. | `internal/store/store.go:1836,1868`, `pgstore/pgstore.go:310-345` |
| Cross-process feed | pgstore already uses `pg_notify('rela_changed', ...)` at commit, one listener connection per process, `rela_seq` watermark catch-up every 30s. `rela_seq` is never exposed past the listener. fsstore has an fsnotify watcher with echo suppression. sqlite is single-process. | `pgstore/feed.go`, `pgstore/listener.go:39,248`, `fsstore/watcher.go` |
| Browser push | `GET /api/v1/_events` SSE, one in-process `eventBroker`, 200ms per-connection coalescing. Frames carry only `entity:changed {type}`. **No entity ids, on purpose** (TKT-POT9GQ), to avoid an existence oracle. SPA invalidates TanStack queries per type. | `internal/dataentry/watcher.go:62-150,407,456-541`, `frontend/src/composables/useEvents.ts` |
| Agent push | None. MCP go-sdk v1.8.0, no resource subscriptions, remote transport is `Stateless: true` (2026-07-28 revision). | `internal/mcp/server.go:552-627` |
| Ephemeral KV | `state.KV`: Get/Put/Delete only. No TTL, no CAS, no scan, no watch. pg variant is shared across nodes. | `internal/state/state.go:23-36` |
| Lease precedent | `schedulerstate`: `StartRun(..., leaseUntil)`, `Reap(now)`, `CreateRun(expectVersion)`, with a pg implementation. `lock.Locker` is blocking, per-operation, not TTL. | `internal/schedulerstate/schedulerstate.go:227-321`, `internal/lock/lock.go:76` |
| Timers | `jobs.Queue` has no delayed/run-at; the package deliberately knows nothing about schedules. | `internal/jobs/jobs.go:46-51` |
| Chat precursor | `comments`: flat thread per `Target{Type, ID, Face}`, out of the graph (no entity, audit, versioning, search), author from the request principal. No threading, no live delivery, no non-entity targets. | `internal/comments/comments.go:379`, `.claude/rules/comments.md` |
| Notifications precursor | `desktopnotify`: `desktop.yaml` rules `{id, type, condition, title, body}` evaluated against `store.Subscribe`, diffed against seen matches. | `internal/desktopnotify/doc.go` |
| ACL batching | `acl.Request.ReadableFacesMany(type, ids)` and `visibility.Resolver.ResolveIDs/ResolveHeaders` exist. `acl.Request` is not safe for concurrent use. The SSE broker already holds one gate per connection and re-derives it after relation changes. | `internal/acl/faceverdict.go:111`, `internal/visibility/batch.go:51,81`, `dataentry/watcher.go:549-590` |
| State machines | Enum `transitions:` compiled by `internal/statemachine`; `internal/predicate` is concurrency-safe and has no I/O. Both are entity-keyed today. | `internal/metamodel/types.go:250-290`, `internal/predicate/doc.go` |
| Principals | `Principal{User, Tool, ...}` plus verified `principal_type`. Agent vs human is inferred from `Tool` and `principal_type`; there is no explicit agent flag. | `internal/principal/principal.go:90-130,391-413` |
| Config | Many top-level YAML files via `config.Loader` with hot reload. Precedent both ways: `desktop.yaml`, `schedules.yaml` (own file) vs `comments:`, `transforms:` (inside `schema.yaml`). configedit allowlist covers only `schema.yaml` and `data-entry.yaml`. | `internal/config/config.go`, `internal/configedit/allowlist.go` |

Conclusion: rela already has a lossy, typed, cross-process change feed and an
ACL-gated per-connection SSE pipe. It lacks ordered events, entity-granular
push, TTL/CAS state, durable timers, and any agent push channel.

### Constraints from the rules

- Read-side ACL goes through `internal/visibility` wrappers, never per-consumer
redaction. A hidden entity is nonexistent: a pushed "removed" must look the same
as a delete. Directus CVE-2023-38503 (subscriptions skipped permission filters)
is the failure mode of a separate realtime read path.
- Aggregates gate before they fold and are per-principal (gantt rule). Live
roll-ups cannot be one shared delta.
- No user Lua on the read path. Subscription filters and session conditions use
`internal/predicate`.
- External side effects (mail, HTTP, AI) go on `jobs.Queue`, not inline.
- Runtime state uses `state.KV`, not local files.
- Comments-style content stays out of the graph: no entity type, audit,
versioning or search.
- No AI on hot paths: a signal that wakes an agent must be quota'd and debounced.
- Capture state once per operation: each push batch is computed from one
snapshot.

### Prior art: realtime platforms

| System | Lesson for rela |
| --- | --- |
| **Hasura** live queries | Group subscribers by (query shape, role); pass identity as a parameter; one SQL query per 100 subscribers; poll + diff at 1s. Visibility changes are handled by re-running. The most direct answer to per-subscriber ACL at scale. |
| **Supabase Realtime** | Postgres Changes checks every event against every subscriber (single-threaded); throughput collapses from 64 to 6 msg/s per client at 500 clients; RLS is not applied to DELETE. The anti-pattern. Broadcast/Presence authorize at join and cache for the connection, so revocation waits for a new token. |
| **Convex** | Read-set invalidation over the transaction log; re-runs queries; all of a client's subscriptions at one timestamp. Matches rela's snapshot rule. |
| **Zero / Replicache** | Permissions compiled into the query via server `ctx`. Replicache "poke": a contentless hint, then the client pulls through its own view. No protected data on the bus. |
| **ElectricSQL** | Server-fixed shape that the client can only narrow; explicit `must-refetch` when continuity is lost. |
| **Ably** | Presence keeps a member 15s after abrupt disconnect; resume within ~2 min, otherwise `resumed: false` tells the client it missed events. |
| **Phoenix.Presence / Tracker** | Presence as owner-replicated CRDT with heartbeat and `down_period`; no central store. Reference design for multi-node presence. |
| **Liveblocks** | Rooms + presence + storage + comments; backend AI agents appear in presence, can be @mentioned, receive room events by webhook. ACL is per room only. |
| **PocketBase** | Reuses the collection's view/list rules for SSE subscriptions. Right instinct, single node. |
| **Postgres LISTEN/NOTIFY** | 8000-byte payload, not durable, commit-ordered. Use as a cross-instance poke, never as the data channel. |
| **Yjs / Automerge** | Document CRDTs; only needed for rich-text co-editing inside one property. Keep outside the graph. |

### Prior art: agent coordination, workflow and chat

| System | Lesson for rela |
| --- | --- |
| **MCP 2026-07-28** | Stateless; one `subscriptions/listen` stream; server-to-client requests replaced by `input_required` results the caller resumes; tasks extension (`working`, `input_required`, `completed`, `cancelled`, `failed`). Sessions and leases must be **handles passed as arguments**, not connection state. |
| **A2A 1.0** | Separates conversation (`contextId`), unit of work (Task) and durable output (Artifact). Subscribe returns a snapshot first, then deltas. Interrupted states `INPUT_REQUIRED`, `AUTH_REQUIRED`. Maps onto session / claimed entity / graph write. |
| **Claude Code agent teams** | Minimal swarm kit: claimable tasks with dependencies, a mailbox per agent, gate hooks. Claims use file locks and get stuck when an agent stops; TTL leases fix that. Agent messages never carry a human's authority. |
| **Temporal / Restate / Inngest** | "Wait for signal X or timeout T, then branch" is the core human-in-the-loop primitive. Temporal separates fire-and-forget signals from validated updates. |
| **AWS Step Functions** | Declarative JSON state machine; `waitForTaskToken` + heartbeat + timeout is enough for human approval. Closest analogue to a YAML session. |
| **BPMN / Camunda** | Candidate-group + claim is a mature human lease. Timers come in two kinds: non-interrupting (remind, escalate) and interrupting (reassign, cancel). |
| **XState / SCXML** | Delayed transitions and parallel regions express per-state timeouts and concurrent roles without code; snapshots are serializable checkpoints. |
| **AutoGen / MS Agent Framework** | Turn-taking is a pluggable policy (round-robin, role order, moderator) over a shared transcript; humans can be ordinary participants. |
| **OpenAI Agents SDK** | Handoff is an explicit, typed transfer of control plus a context filter. |
| **Leases** (k8s Lease, etcd, Chubby, SQS) | Lease = holder + TTL + renewal + **fencing token**. Without fencing, an expired holder that wakes up can still write. |
| **Blackboard / Linda** | rela's graph already is a blackboard. Predicate subscriptions are the triggers; an atomic "take" is a lease claim. Agents need no addressing. |
| **Matrix** | Room = durable state events (keyed, last wins) + timeline + explicitly ephemeral EDUs (typing, receipts, presence). Permissions per event type, declared in data (`power_levels`). |
| **Zulip** | Every message has a topic. In rela the natural topic is an entity id. Resolving a topic is a signal. |
| **Slack AI apps** | Agent status ("searching...") and suggested prompts are first-class ephemeral signals; interactive messages bind buttons to typed actions. |
| **Discord** | Threads auto-archive after inactivity: a cheap default lifecycle for ephemeral sessions. |
| **incident.io / PagerDuty / Rootly** | The reference domain. War room is ephemeral; pinned moments are explicitly promoted into a durable timeline; roles are leases; workflows are declarative trigger/condition/step rules; timed escalation tiers. |
| **Jira SM / Linear** | SLAs are timers with start, pause and stop conditions over entity state, not one-shot sleeps. |

### Use cases

Key: Sub = live subscription, Pres = presence, Lease, Sig = signal, Sess =
session, Chat, Stream, Hand = handoff.

| # | Scenario | Primitives | Durable (graph) | Ephemeral |
| --- | --- | --- | --- | --- |
| 1 | Chat about an entity (topic = entity) | Chat, Sub, Pres | Decisions promoted into the entity | Messages (log), typing, viewers |
| 2 | Agent swarm claims ready tickets | Sub, Lease, Sig | Assignee, transitions, results, audit | Heartbeats, "working on X" |
| 3 | Incident / data-breach war room with GDPR 72h deadline (atlas `datalek`) | Sess, Lease (roles), Chat, Sig, timers, Hand, Stream | Breach entity, timeline entries, deadline, notification sent, role holders | Chatter, presence, agent drafts |
| 4 | Approval with SLA escalation | Sess, Sig, timers, Sub | Decision, approver, escalation events | Reminders, receipts |
| 5 | Agent asks a human mid-task (`input_required`) | Sig, Sess, Hand | The answer if it changes data | Pending question UI |
| 6 | Live meeting capture (atlas `mt_overleg`) | Sess, Stream, Pres, Chat, Sig | Meeting, decisions, action items | Transcript, draft notes |
| 7 | Co-editing a form (DPIA, risk) | Pres (field focus), Lease (field), Sub | Saved values via `PatchEntity` | Cursors, uncommitted drafts |
| 8 | Support triage | Sub, Lease, Hand, Chat, timers | Classification, assignment, SLA breaches | Drafts, agent status |
| 9 | Agent-to-human escalation and back | Hand, Sess, Chat, Pres | Handoff record (from, to, reason) | Live conversation |
| 10 | Shift handover | Sess, Hand, Lease, Sig (ack) | Handover record, ownership, ack | Briefing |
| 11 | Live dashboard / kanban | Sub | Nothing new | Pushed deltas |
| 12 | Pair review (human + agent reviewer) | Sess (turns), Chat, Sig, Pres | review-response entities | Discussion, current item |
| 13 | Onboarding checklist with buddy and agent guide | Sess, timers, Sig, Sub | Item completion, dates | Nudges, Q&A |
| 14 | Decision vote with quorum and deadline | Sess (quorum), Sig (vote), timers | Decision, votes, rationale | Deliberation |
| 15 | ISMS control sweep with exceptions to owners | Sub, Lease, Sig, Hand, timers | Findings, NCRs, responses | Sweep progress |

Across all fifteen: **assignments, decisions, deadlines, role transfers and
handoffs are always durable**; that is what auditors and regulators ask about.
**Presence, typing, agent status, heartbeats and draft text are always
ephemeral.** Chat sits between: retained as a log, entering the graph only by
explicit promotion.

Frequency of primitives: Sub (12), Sig (11), Sess (10), Lease (7), Chat (7),
Pres (7), timers (7), Hand (5), Stream (3). Subscriptions and signals are the
foundation; sessions are the most-used composite.

## Options

The options split into one structural choice (how realtime relates to the graph)
and four sub-decisions that apply to whichever structure wins.

### Structural choice

#### Option S1: Everything is an entity

Sessions, leases, messages and presence are entity types. Realtime adds only
live subscriptions over the graph; sessions reuse enum `transitions:` and
automations.

- Pros: no new concepts; ACL, audit, views and MCP work unchanged; sessions are
visible in every existing tool.
- Cons: heartbeats and presence become audited, versioned writes (churn,
history noise, write amplification on fs/git); contradicts the comments rule
that content-about-content stays out of the graph; leases need CAS that the
write path does not offer; timers still need a new service.
- Effort: small for live views, then grows badly.

#### Option S2: Separate realtime sidecar

A self-contained subsystem with its own config, stores, transport and protocol.
It touches the graph only to write outcomes through `entitymanager`.

- Pros: clean isolation; the graph stays untouched; can evolve fast.
- Cons: a second read path, which is exactly the Directus failure mode; risk of
a parallel ACL model; live views (the most-used primitive) are inherently graph
reads, so they cannot live in a sidecar anyway.
- Effort: large.

#### Option S3: Layered (recommended)

Three tiers with one rule per tier:

1. **Graph tier (durable).** Live subscriptions over existing views and
queries, gated through `visibility`. Session outcomes, role assignments,
decisions and handoffs are ordinary entity writes via `entitymanager`.
2. **Coordination tier (short-lived, consistent).** Leases and timers. Needs
CAS and expiry, so it gets a dedicated store following the `schedulerstate`
pattern (lease table + reaper, pg implementation for multi-instance), not plain
`state.KV`.
3. **Interaction tier (ephemeral or log).** Presence, signals, messages,
session runtime state. Kept out of the graph like comments. Each signal or
message type declares a persistence tier: `ephemeral`, `log` or `graph`
(Matrix's state/timeline/EDU split).

- Pros: every primitive lands where its consistency and audit needs are met;
reuses the visibility wrappers so there is one read path; matches what incident
tools and A2A do (ephemeral context, durable artifacts).
- Cons: more moving parts; needs a clear rule for what crosses from tier 3 into
tier 1 (answer: only explicit, audited promotion actions or session outputs).
- Effort: large overall, but delivered in independent phases.

### Sub-decision A: Fan-out and ACL

| Option | Description | Verdict |
| --- | --- | --- |
| A1 Per-event per-subscriber check | Supabase Postgres Changes, PocketBase | Reject: proven not to scale, easy to get DELETE wrong |
| A2 Authorize at join only (rooms) | Liveblocks, Ably, Pusher | Use only for interaction-tier channels (session, entity chat), where the room's subject entity is the gate. Re-check on relation/ACL change, as the SSE broker already does. |
| **A3 Poke, then re-read through visibility, batched** | Replicache poke + Hasura multiplexing + Convex re-run | **Recommend for live views.** The bus carries only "topic changed, seq N". Each subscription re-evaluates its view through `visibility.Reader`, batched by (view, principal) with `ReadableFacesMany`, coalesced per window, diffed, then pushed. Removals look the same as deletes. A poll + diff fallback works on every backend. |

A3 also lifts the TKT-POT9GQ restriction safely: frames may name entity ids
because every id in a frame comes from the subscriber's own gated re-read.

### Sub-decision B: Where it is declared

This is open by request. Three options:

| Option | Description | Pros | Cons |
| --- | --- | --- | --- |
| B1 `realtime:` block in `schema.yaml` | Like `comments:` and `transforms:` | One file; signals can be referenced by automations directly | `schema.yaml` grows further; mixes durable shape with runtime behaviour; every key must be classified in the configedit allowlist |
| B2 Top-level `realtime.yaml` | Like `desktop.yaml`, `schedules.yaml` | Own loader, validation and hot reload; default-locked for Configure; clear ownership | Cross-file references need validation; two places to look |
| B3 Split by nature | Interaction **types** (signal types, session types, message types) in `schema.yaml` because they are metamodel; **runtime policy** (TTLs, transports, quotas, triggers) in `realtime.yaml` | Mirrors the existing metamodel / runtime-config split | The split line will be argued per key |

Lean: **B2, with a one-way dependency rule**: `realtime.yaml` may reference
entity types, views, roles and predicates from the rest of the config;
`schema.yaml` never references `realtime.yaml`. Entity-change-to-signal triggers
are declared in `realtime.yaml` ("when an entity matching P changes, emit S"),
so automations never need to know signals exist. This keeps the graph metamodel
unaware of realtime, which makes realtime removable and keeps the configedit
surface unchanged until it is deliberately added. Decide this before phase 3;
phases 1 and 2 need almost no config.

### Sub-decision C: Session runtime

| Option | Description | Verdict |
| --- | --- | --- |
| C1 Session is an entity with an enum state machine | Reuse `transitions:` and automations | Good for long-lived, audited sessions (an incident). Weak for short interactive ones; no turns, roles or timers. |
| C2 Code-defined sessions (Lua or Go) | Like Temporal | Rejected: not declarative, and Lua stays at write time. |
| **C3 Declarative statechart, runtime state outside the graph** | ASL/XState-style YAML: roles, states, `on:` signals, `after:` timers (interrupting and non-interrupting), `when:` predicates, `outputs:` that write to the graph. Compiled with `predicate`, reusing `statemachine` where it fits. A session is a **handle** (id) with a subject entity. | **Recommend.** Long-lived audited sessions can still put their subject in the graph (C1 for the subject, C3 for the interaction). |

Session states should include the A2A/MCP interrupted state `input_required` so
an agent's request for human input maps onto the standard MCP tasks status.

### Sub-decision D: Chat

| Option | Description | Verdict |
| --- | --- | --- |
| D1 Extend `comments` | Add threading (parent id), live delivery, mentions | **Recommend for entity-anchored chat**: same "content about content" rule, same storage backends, same authorship guarantee. |
| D2 New message store | Topics not limited to entities | Needed for session logs, which are not entity-anchored. Share the comments store design; target becomes `Entity` or `Session`. |

In practice: one message model whose target is an entity or a session, built by
generalizing comments. Promotion of a message into the graph (a decision, a
timeline entry) is an explicit, audited action, never automatic capture.

### Sub-decision E: Transports

- **Browser:** keep SSE; add subscription multiplexing on the existing
`_events` connection (or a sibling endpoint), with client-to-server requests
over normal HTTP. WebSockets are not needed; h2c already avoids the connection
limit.
- **Agents (MCP):** implement `subscriptions/listen` when the go-sdk supports it;
until then, a `watch` tool that long-polls with a resume cursor. Leases,
sessions and signals are tools taking handles. Agent input requests map to
`input_required`.
- **Later:** A2A as an alternative agent surface (Task = claimed work, Artifact
= graph write); outbound webhooks for signal delivery (on `jobs.Queue`).

### Multi-instance seam (design now, build later)

- A narrow `Bus` interface, defined at the consumer: `Publish(topic, poke)` and
`Subscribe(topic)`. Implementations: in-process (day one), Postgres NOTIFY on a
second channel next to `rela_changed`, external broker later if needed. Pokes
only; never entity content on the bus.
- **Ordered events with resume.** Expose `rela_seq` (pg) and an equivalent
monotonic counter (fs/sqlite/mem) on `store.Event`. Clients resume from a cursor
within a bounded window; outside it they get an explicit `must-refetch` (Ably
`resumed: false`, Electric `409`).
- **Presence:** each instance owns the presence of its own connections and
replicates heartbeats over the Bus (Phoenix.Tracker model); entries expire after
a grace period. No shared store needed.
- **Leases and timers:** durable CAS rows with a reaper; the pg implementation
is shared by all instances. Leases return a monotonically increasing **fencing
token**; phase 2 adds optional token checking in `PatchEntity`.
- **Interaction-tier stores** (messages, session state) follow the comments
pattern: file, sqlite and pg implementations, with pg shared.

### Security considerations

- Existence oracles: removal equals delete; no pre-filter counts; no reason on
removal; coalescing windows also blur timing.
- Revocation mid-subscription: re-derive the gate on relation and ACL changes
(existing broker behaviour) and on a periodic re-check.
- Agent messages never carry human authority. A signal or message from an agent
cannot approve a human gate (Claude Code teams rule).
- Who may emit or receive a signal type, join a session role, or take a lease is
declared and enforced by `acl.yaml` permissions, compiled to allowlists (no
runtime deny).
- Limits: subscriptions per principal, presence payload size, signal rate per
type, agent wake-up quota and debounce.
- Audit: lease **conflicts** and **handoffs** are audited; heartbeats and
presence are not.

## Recommendation

Adopt **Option S3 (layered)** with A3 fan-out for live views, C3 declarative
sessions, D1+D2 chat on a generalized comments model, SSE plus MCP for
transport, and the Bus / sequence / lease seams designed for multi-instance from
the start. Lean B2 (`realtime.yaml`, one-way references) for configuration, to
be decided before phase 3.

Phasing, each phase shippable on its own:

1. **Live views.** Sequence numbers on `store.Event`; `Bus` interface with an
in-process implementation; per-subscription SSE frames via poke + gated re-read,
batched per principal; resume cursor with `must-refetch`; MCP `watch` tool.
Replaces "entities of type T changed" with precise, ACL-safe deltas.
2. **Presence and leases.** Lease store with CAS, TTL, heartbeat, fencing token
and reaper (schedulerstate pattern); presence with grace period; agent status
signal. Dogfood on the tickets project: agents claim tickets instead of
colliding.
3. **Signals and chat.** Signal types with persistence tiers and declared
emit/receive permissions; entity-change triggers; generalized comments as
messages with threads and live delivery; explicit promote-to-graph action.
Settles the config-location decision.
4. **Sessions.** Declarative statecharts with roles, turn policies, interrupting
and non-interrupting timers, `input_required`, handoffs, and graph outputs.
Durable timer service.
5. **Multi-instance.** Postgres Bus, replicated presence, shared lease and
timer tables (mostly already designed into phases 1 to 4).

Trade-offs accepted:

- More moving parts than S1, in exchange for not putting heartbeats and chatter
into an audited, versioned graph.
- Live-view re-reads cost more server CPU than shipping raw events, in exchange
for one ACL path and no existence oracles. Batching per principal bounds the
cost; a budget test pins it (collection-reads rule).
- Sessions are a new declarative language. It is kept small (roles, states,
signals, timers, predicates, outputs) and borrows ASL/XState semantics rather
than inventing new ones.
- No CRDT co-editing. Field presence and field leases plus `PatchEntity` cover
form co-editing; rich-text CRDT is out of scope until a concrete need appears.

Open questions:

1. Config location (B1/B2/B3), to settle before phase 3.
2. Does a lease's fencing token become mandatory for writes to a leased entity,
or advisory? (Mandatory is safer but touches every write path.)
3. Should an explicit `agent` principal kind be added, or is `principal_type`
plus `Tool` enough to apply agent quotas and the "no human authority" rule?
4. Session log retention: per session type, and where it lives on the fs
backend (state store vs files).

## Follow-up decisions: channels (2026-10-11)

Discussion after the first recommendation refined sub-decision D (chat) and
folded signals and sessions into one concept. These supersede the earlier text
where they conflict.

1. **Channels and messages are outside the graph.** Neither is an entity. This
keeps the graph from growing with conversation volume, keeps chatter out of
audit and versioning (TKT-FIO205 applies), and avoids per-message write-path
cost.
2. **A channel is anchored to an entity.** Its identity is
`(channel_type, entity ref)`, plus a name when a type allows several per entity.
Standalone channels (company-wide, team) anchor to a team, project or workspace
entity, so access derivation stays uniform.
3. **Channel types declare typed message schemas.** Each message type has
properties using the metamodel's property types (string, markdown, date, enums
from `schema.yaml`, `ref`), a `post` permission, a persistence tier (`ephemeral`
or `log`), optional `actions`, and an optional `promote_to` mapping into an
entity type. Validation at post time is declarative only: types, required, size,
`ref` targets checked through `visibility`, `when:` predicates. No Lua,
automations or audit per message.
4. **Read access is per channel only.** Everyone who can read a channel sees
every message in it. No per-message-type read rule, no field redaction in
messages. Sensitive content goes in a sibling channel type on the same entity
(for example `incident-room` and `incident-legal`). Reason: hidden messages
inside a shared conversation confuse participants ("why can't I see what you are
discussing?") and per-type read rules leak through sequence gaps. The one
unavoidable per-reader difference is entity references, which render as plain
text for readers who cannot see the referenced entity.
5. **Post permissions per message type stay.** Restricting who may post a type
or trigger an action does not create divergent views.
6. **Signals are message types with `persist: ephemeral`.** Typing, presence,
agent status, `mentioned` and `needs-human` are message types in a channel.
7. **Sessions are channel types with a statechart.** Typed messages are the
events that drive transitions; session outputs go to the graph through
`promote_to` and action handlers.
8. **Agents post typed messages.** MCP exposes one JSON schema per message
type, so agent coordination (claim, progress, result, needs-human) is validated
and permissioned instead of free text.
9. **Messages record a schema hash.** Logs are immutable history; old messages
keep their shape and render leniently. Data migrations do not apply to channel
logs.
10. **Membership changes are audited in the channel store**, since channels are
not in the graph.

Typed channels make the config a language of its own that references schema
types, which strengthens the lean towards a top-level file (`realtime.yaml` or
`channels.yaml`) with one-way references.

## Follow-up decisions: channel lifecycle (2026-10-11)

Mockups: `frontend/packages/rela-components`, Storybook "Mockups/Realtime"
(stories `EntityMultipleChannels*` and `Lifecycle*`).

1. **An entity can carry several channels.** A channel type declares
`cardinality: one` (the incident room) or `cardinality: many` (topics anyone
permitted can start with `create:`). The entity page lists the channels the
viewer can read. A channel the viewer cannot read is not listed at all.
2. **Three states plus deletion.** `open` takes posts under the message-type
rules. `closed` is read-only: no posts, message actions and timers stop, and it
stays on the entity page. `archived` is read-only and shown only behind "Show
archived". `delete` removes the messages and leaves a deletion record in the
channel store's audit log.
3. **Every lifecycle operation is declared by the operator.** `close`,
`reopen`, `archive` and `delete` sit under `lifecycle:` per channel type. An
undeclared operation is not offered (default-locked). Each one names who may run
it (`roles:` or `permission:`), when (`condition:` predicate, `from:` state),
and the text people see when they may not (`denied:`). `auto:` lets a channel
follow its entity or a timer, for example close when the breach closes, archive
30 days after closing.
4. **Retention is operator-configurable.** `retention:` blocks deletion for a
period counted from closing.
5. **Entity deletion versus retention is operator-configurable.**
`on_entity_delete:` per channel type chooses between `delete` (the channel goes
with the entity), `detach` (the channel is kept archived until its retention
ends) and `block` (the entity cannot be deleted while a retained channel
exists).
6. **Message operations are a separate, operator-configurable set.** Editing or
deleting your own message and removing someone else's message need different
permissions, so each is declared under `message_ops:` with its own roles or
permission, an optional window (for example edit within 15 minutes), and
`denied:` text. A removed message leaves a "message removed by X" placeholder in
the log, so sequence numbers and replies stay consistent.
7. **Menus list channels through a filter.** A `data-entry.yaml` menu entry
takes `channels:` with filters such as `following: true` and `state: open`.
8. **Every transition is visible and audited.** It posts a system message in
the channel and writes an audit entry in the channel store. Agents follow the
same rules through their role.
