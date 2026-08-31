---
id: TKT-1EM4KL
type: ticket
title: 'Declarative webhook routes: map an inbound HTTP request onto entity create / find-or-create / update'
kind: enhancement
priority: medium
effort: l
tags:
    - needs-design
status: ready
---

Let an operator declare, in config, an HTTP endpoint that maps an inbound
request onto entity writes — without writing Lua. The common integration shapes
(a monitoring alert, a form post, an upstream event) are all the same small
pipeline, and the machinery for most of it already exists.

Sequenced FIRST; the Lua escape hatch ([[TKT-EFMRQM]]) follows and handles what
the declarative vocabulary cannot express.

## Three workflows, not one

Uniqueness is a property of the **operator's schema**, not something the webhook
layer may assume. Three distinct shapes must all be expressible:

1. **Always create** — webhook -> new entity, properties and body from the
payload via a template. No matching, no key. (An inbound form post; an
append-only event log.)
2. **Find-or-create** — look up an entity, create it from a template if absent,
then mutate it. Needs an identity key.
3. **Find and update only** — no create, so no create race.

Only (2) needs a dedup key. Requiring one everywhere would break (1) outright,
so `find:` is optional and the key is required only when `create_if_missing` is
present.

## Sketch

The identity key is a **computed property on the entity**, not something the
webhook config hashes — see Identity below.

```yaml
# schema.yaml
incident:
  properties:
    alert_key:
      type: string
      unique: true
      computed: sha256(entity.host .. "/" .. entity.service)
```

```yaml
# data-entry.yaml — the hook id IS the URL segment: /hooks/icinga-alert
webhooks:
  icinga-alert:
    find:
      type: incident
      match: [host, service]      # names properties; alert_key is derived
    create_if_missing:
      template: incident
      properties:
        title: "{{body.host}}/{{body.service}}"
        host: "{{body.host}}"
        service: "{{body.service}}"
        status: open
    then:
      - append_section:
          section: Notifications
          content: "- {{now}} **{{body.state}}** — {{body.output}}"
    respond: { status: 200 }
```

**Route naming is settled: the hook id IS the URL segment.** A hook needs an id
in the YAML regardless, so `icinga-alert` serves `/hooks/icinga-alert`. No
`path:` key — an explicit path adds an aliasing surface to design, validate and
keep consistent with the id, for no gain. Aliasing, if ever wanted, is a YAML
anchor or a later version.

## What already exists vs. what is new

Reuse, do not reinvent:

- **`create_entity` + templates.** `autocascade.CreateEntityOptions` already
carries `TemplateVariant`, `Properties`, `Content` — that is workflow (1) almost
entirely.
- **`{{...}}` interpolation.** The automation vocabulary has `{{new.x}}`,
`{{today}}`, `{{entity.id}}`; this adds a `{{body.*}}` / `{{query.*}}`
namespace.
- **`internal/filter`** for any `where:`-style selection.
- **`unique:`** for the identity key — see below.

Genuinely new:

- **Find-by-key has no declarative equivalent.** Today's `if_exists` resolves
existence via `autocascade.Host.FindExistingRelationTarget` — "is there an
entity at the end of this relation *from the trigger entity*." A webhook has no
trigger entity and no relation to walk, so find-or-create as it exists cannot
express this.
- **`append_section` is not a write primitive.** Lua has
`rela.md.extract_section` (read-only, `internal/lua/markdown.go:135`). Appending
to a named section means parse -> locate heading -> splice -> serialize ->
`PatchEntity` with `Content`. Useful well beyond webhooks.
- **A `sha256` host function in the predicate stdlib**
(`internal/predicatefns` has match/regex/fuzzy/contains/len/today today). This
is the ONLY new expression machinery needed — see Identity below.
`internal/lua/crypto.go` already exposes `sha256_hex`, so the primitive exists.
- **Route registration from config**, and a `respond:` shape.

## Identity: a computed, unique property (settled)

The operator names the fields; **rela derives the key**. `computed:` already
does exactly this — a pure entity-local expression materialized on every write,
read-only, evaluated in dependency order — and computed values are "stored and
indexed exactly like authored properties", so a computed property can carry
`unique:` and get the postgres derived index with no special casing.

```yaml
alert_key:
  type: string
  unique: true
  computed: sha256(entity.host .. "/" .. entity.service)
```

Three reasons this beats hashing in the webhook config:

- **The key belongs to the entity, not the webhook.** Every writer — webhook,
  Lua, CLI, sync — gets the same identity. A webhook-local hash would leave the
  invariant unenforced on every other path.
- **No new template syntax.** `{{...}}` is substitution-only, so a
  `{{sha1 ...}}` form would need call syntax. Naming a property instead reduces
  the whole extension to ONE host function (`sha256`) in the predicate stdlib —
  the seam built for exactly this.
- **Migration is already handled**: changing a `computed:` reports schema drift
  and `rela migrate gen` drafts `recompute_computed`.

One consequence to design around: computed values are materialized on WRITE, so
`find:` matches on stored values. The webhook therefore sets the SOURCE fields
(`host`, `service`) and rela derives the key — not the reverse.

## Concurrency: conflict detection, not locking (settled)

**No lock is needed.** A lock keyed on entity identity would be circular anyway
— a request cannot lock an entity before finding it, and on create there is no
entity to lock. (A lock on the payload-derived NATURAL key sidesteps that, since
it is computable at t=0, but conditional writes make the whole question moot.)

Both halves of the pipeline are conflict-detecting with mechanisms that already
exist:

| | Conflict mechanism | Loser sees | Needs a lock? |
| --- | --- | --- | --- |
| **Create** | `unique:` index (pg) / pre-write scan | `store.UniquePropertyError` | **No** |
| **Append** | `If-Match` vs. content-hash ETag | `412 Precondition Failed` | **No** |

**Create is already solved without a lock.** `Manager.CreateEntity` writes
directly and surfaces the conflict — *"No GetEntity pre-check: it was a TOCTOU
duplicate of the store's atomic uniqueness guarantee."* The loser re-finds. That
is a conditional insert in effect: atomic, existing, and working on the fs tier
where there is no advisory lock.

**Append is ALSO solved by conflict detection — the mechanism already exists.**
`internal/dataentry`'s write handler implements optimistic concurrency:
`If-Match` against `computeEntityETag` (`api_v1.go:2030`), a sha256 over id,
type, content, sorted properties and outgoing relations; a mismatch is 412.
(It lives in the HTTP layer, not `entitymanager` — easy to miss when looking
only at the manager.)

So the whole pipeline is conflict-detecting with **no lock**:

```text
find -> miss  -> create with unique:  -> loser gets UniquePropertyError -> re-find
find -> hit   -> PATCH with If-Match  -> loser gets 412                 -> re-find, re-apply
```

The ETag is a CONTENT HASH, which is what makes this safe without a
cross-process transaction: a concurrent writer changes the content, so the
loser's stale ETag cannot match. It is a compare-and-swap, not a timestamp
comparison, so it cannot miss an update the way a coarse-resolution
`updated_at` can. Within a process the read-compare-write is additionally
atomic under `writeMu`.

Why this is preferable to the keyed lock here:

- Works on **every tier** (fs, sqlite, postgres). The lock only spans processes
  on postgres.
- **No held connection.** The pg lock pins a pooled connection for its whole
  lifetime, so a lock leak is a connection leak.
- **No acquire timeout to tune**, no lock-ordering or deadlock surface.
- Under a burst, losers fail fast and retry rather than queueing — better when
  contention is rare, which it is.

The cost is a **bounded retry loop** in the webhook executor: catch
412 / `UniquePropertyError`, re-find, re-apply, cap the attempts. That is local
to this pipeline rather than a new primitive.

Caveat worth designing around: a retry re-runs the whole step. That is fine for
an idempotent append (re-read the body, re-append) and wrong for a step with
other side effects — an argument for keeping declarative steps pure.

**[[TKT-1K47YD]] (the keyed lock) is therefore NOT a dependency of this ticket.**
It stands on its own as a capability rela lacked, and remains the right tool
when a critical section genuinely cannot be expressed as a conditional write.
This pipeline can be.

Still worth having: a **server-side append mode on `entity.Patch`** (the manager
appends to `Content` instead of the caller doing read-modify-write) would remove
even the retry, on every tier. Larger change, touches the write path, useful
beyond webhooks — a candidate follow-up, not a v1 requirement.

## Why the index still matters (investigated)

`checkUniqueProperties` is a **check-then-write, not an atomic constraint**
(`internal/entitymanager/unique.go:38-51`) — scan and write with no lock across
them, so racing writers can both pass. On **PostgreSQL** a derived partial
unique index (TKT-3Q0GP1) backstops it: the loser fails atomically as
`store.UniquePropertyError`, mapped to the same 422. **The index closes the
race.** fsstore/memstore have no index but are single-writer by nature.

So: **when the operator declares the match property `unique:`, find-or-create is
race-free on postgres.** When they do not, it still works but can duplicate
under concurrent delivery. The vocabulary should *offer* the safety, not mandate
it — and the docs must say plainly which guarantee applies, rather than implying
one that isn't there.

Follow the existing precedent for the write itself. `Manager.CreateEntity`
carries: *"No GetEntity pre-check: it was a TOCTOU duplicate of the store's
atomic uniqueness guarantee."* A declarative find-or-create should likewise
**attempt the create and handle `UniquePropertyError` by re-finding**, not
reintroduce check-then-write. `UniquePropertyError` carries `Property` (not the
value), but the caller supplied the value, so the re-find is straightforward.

A transactional find+create is possible in principle — `store.Store.Tx`
(DEC-8UIL0) serializes cross-process on pg — but is the wrong tool here: `Tx`'s
contract forbids slow external I/O inside `fn` ("the whole deployment's writers
wait"), and `Manager` uses `Tx` in exactly one place (delete/cascade) today. No
new transactional write API should be needed.

## What Icinga actually provides (researched)

Relevant because it decides what a key can be built from:

- **No per-notification unique ID exists.** The runtime macro table exposes only
`notification.type` / `.author` / `.comment`; `notification.cpp` mints no
per-execution id; none of the 15 API event types carry an event id. There is no
`jti` equivalent.
- **Icinga's own answer is a content hash, built by the consumer.** Icinga DB's
`notification_history.id` is `sha1(environment.name + notification.name + type +
send_time)`, explicitly so two HA masters don't duplicate the row. Declaring a
computed hash `unique:` is therefore the same solution upstream uses, not a
workaround.
- **Icinga does not retry** — `notification.cpp` executes the command once, logs
an exception, does not re-queue. **The primary failure mode is under-delivery,
not over-delivery.**
- **Duplicates still occur from HA** (icinga2 #4647, #10623): two masters send
byte-identical notifications, distinguishable only by content hash. So dedup is
still needed — for HA, not for retries.
- **No episode identifier.** Nothing ties PROBLEM -> ACK -> RECOVERY together.
`last_state_change` is the usual homegrown key, but a flap
(CRITICAL->WARNING->CRITICAL) moves it and fragments one operator-visible
incident. Icinga's own fix is a separate product (Icinga Notifications) with
first-class incidents keyed on tag sets.
- For contrast, Alertmanager ships `fingerprint` (fnv64a over sorted labels) and
`groupKey`, retries with backoff, and reuses the fingerprint across
firing/resolved — lifecycle correlation for free. A rela design should not
assume Icinga's guarantees are the floor.

Implication: two *different* keys may be wanted — delivery-dedup (catches HA
duplicates) and episode-identity (which incident to append to). Whether the
vocabulary exposes both, or one, is a design question.

## Why concurrency drove the design (researched; superseded by the lock above)

**Icinga dispatches notifications concurrently, with no cap.**
`notification.cpp:498` posts every notification to a process-global
`boost::asio::thread_pool` sized `hardware_concurrency() * 2`. Multiple
users/usergroups dispatch in parallel (the loop posts, it does not call);
nothing serializes between checkables; and `Process::Run` returns after the
fork, so a pool worker does NOT block for the request duration — concurrency is
bounded by forked OS processes, not pool width. There is no
`max_concurrent_notifications` (`MaxConcurrentChecks` is consumed only by the
checker component). The only knob is global `Configuration.Concurrency`, which
would throttle the entire daemon.

200 services CRITICAL x 3 users is readily **hundreds of concurrent POSTs**.

Against rela's `writeMu` — ONE process-wide mutex held for the whole of script
execution (`internal/dataentry/write_handler.go`) — those serialize into a
queue. At ~200ms per script the 300th request waits a minute, the
`NotificationCommand` timeout kills it, and because **Icinga never retries that
alert is permanently lost**.

So the ranking is:

1. **Throughput collapse under a burst — the real failure mode.** Produces
   SILENT DATA LOSS. Note TKT-X06LA2 was "fix the writeMu DoS" on this very
   surface; a webhook endpoint is where a third party controls the rate.
2. **Create race — real but narrow, and cheap to guard.** Closed within a
   process by `writeMu`; across processes (postgres multi-writer, several
   rela-server processes on one database) by the `unique:` index.

The mitigation both the research and CLAUDE.md point at is **accept-and-queue**:
validate cheaply, return 202, process on `jobs.Queue` — external side effects
belong there rather than inline on a write path. Caveat: the fs/desktop jobs
tier is deliberately ephemeral, so durable accept-and-queue is postgres-only,
and a 202 must not be read as a durability promise on fs.

### Option: an operator-declarable KEYED webhook lock (postgres)

A third option, and probably the better fit: let the operator declare a lock on
the webhook, **keyed** from the payload, backed by a postgres advisory lock.

Keying is the crucial part. A *global* webhook lock reproduces the `writeMu`
problem at database scope (worse — it would serialize across processes too).
Keyed on e.g. `host+service`, a 200-service outage runs 200 locks in parallel
and contends only where two notifications genuinely concern the same incident —
which is exactly where serialization is wanted.

One mechanism then closes both correctness problems without a queue:

- **Create race** — two HA duplicates take the same key; the loser waits and
  then FINDS the entity the winner created.
- **`append_section` clobber** — the case `unique:` cannot help with, because
  nothing is being created. The lock does.
- **Throughput** — unaffected for distinct keys, which is the burst case.

rela already has the machinery and the idiom: four distinct advisory-lock keys
(`migrate`, `reconcile`, `migration`, `sweep`) using
`pg_try_advisory_lock($1::int, hashtext(current_schema()))` — a two-int lock
whose second slot is already schema-scoped for tenant isolation. A webhook lock
is the same shape with the second slot as the payload-derived key.

Three things to get right, all visible in the existing callers:

1. **Session-scoped: the whole operation must run on ONE acquired connection.**
   `purge.go` and `sweep.go` both do this; the sweep's doc warns that issuing
   the work via the pool (other sessions) "silently voids the single-writer
   guarantee."
2. **Blocking, not try.** Every existing caller uses `pg_try_advisory_lock` and
   SKIPS on failure — correct for a sweep, wrong here, where skipping means
   losing an alert. This needs blocking `pg_advisory_lock` with a bounded wait,
   which is a new pattern and needs its own timeout story.
3. **Postgres-tier only.** fs/sqlite have no equivalent, but `writeMu` already
   serializes those globally, so this is a refinement of the strong tier rather
   than a portable primitive.

Tension to resolve: `Tx`'s contract forbids slow external I/O inside `fn`, and a
lock held across a Lua script that makes an HTTP call has the same hazard. The
lock should scope the **entity mutation**, not the whole request.

Whether v1 does accept-and-queue, a keyed lock, or ships synchronous with a
documented throughput ceiling is a design decision — but it should be made
deliberately, not by default. The keyed lock looks the strongest: it fixes
correctness AND leaves burst throughput intact, where accept-and-queue defers
the work and a synchronous v1 fixes neither.

## Delivery loss is the operator's to solve (settled: no rela mechanism)

**Decision: docs, not machinery.** rela does not persist-then-process. The keyed
lock removes the throughput pressure that originally argued for
accept-and-queue, and a bounded lock wait absorbs the common near-simultaneous
case. What remains — a rela restart or outage mid-delivery — is not something
rela can fix from the receiving end, and a producer that does not retry will
lose the alert whatever rela does.

This matches the existing precedent that mail is "notification, never a system
of record". Operators needing guaranteed capture should poll the Icinga API
event stream or Icinga DB rather than rely on push.



Because Icinga does not retry, a 5xx or a restart mid-delivery loses the alert.
rela cannot fix that, but the operator can: a NotificationCommand is just a
script Icinga executes, so one that POSTs with retry (`curl --retry`, or a
wrapper) closes the gap sender-side. Document this; do not engineer around it.
For guaranteed capture, polling the API event stream or Icinga DB is the right
answer, not push.

This matches the existing precedent that mail is "notification, never a system
of record".

## Design questions

- **One key or two** (delivery-dedup vs. episode-identity). The computed-key
approach handles episode identity; whether a separate delivery-dedup key is
also wanted — to swallow byte-identical HA duplicates rather than append twice
— is still open. Note the lock serializes them but does not make the second a
no-op.
- **`append_section` semantics**: missing section — create it, or error? Where
in the body? (The concurrency half is settled — conditional writes, see
Concurrency above.)
- **Retry budget**: how many times the executor re-finds and re-applies after a
412 / unique conflict before giving up, and what it returns when it does.
- **`sha256` in the predicate stdlib**: exact spelling and whether the digest is
hex or base64 — it becomes a stored, indexed, unique value, so the encoding is
effectively permanent once projects depend on it.
- **Body size cap and content types.** The action endpoint has none today; a
parsed body becomes a Lua/Go structure, so a cap bounds real memory.
- **Header exposure** — allowlist, never pass-through; headers carry cookies,
bearer tokens and proxy assertions.
- **Failure response**: which status a validation failure vs. a script failure
returns, given the sender may treat 4xx and 5xx differently.

## Out of scope

- Any producer authentication scheme in rela. The proxy in front (Pratique,
oauth2-proxy) terminates that and hands rela an ACL-bounded request.
- Outbound webhooks (rela calling out on graph change).
- Any Icinga-specific Go code — an Icinga mapping is config plus, if needed, a
Lua script.
