---
id: TKT-8P1TM7
type: ticket
title: 'Sync becomes a client of the authorized API: read + write through /api/v1, retire the private sync record channel'
kind: enhancement
priority: medium
effort: l
status: done
---

## Problem

Sync (`/api/sync/`, FEAT-NJ9FEN) is a **private second channel into the store**,
parallel to the authorized `/api/v1` API the SPA uses. It evaluates
authorization on its own, and diverges from `/api/v1` in two security-relevant
ways:

1. **Read.** `handleSyncGet` returns each record's **full canonical body** — all
entity properties, all relation meta — gated only by the **row-level** read
verdict, NOT `visible:` field/meta redaction. A principal with ordinary reader
access pulls redacted values straight off the sync GET. (Entity fields since
TKT-73C6B2; relation meta since TKT-B1F5Q1. Surfaced by RR-FOD7IB.)

2. **Write.** `handleSyncPut` applies via `ApplyEntity`, which runs **neither**
field-write gate — not the SPA handler's `validateFieldWrite`, not the manager's
`CheckFieldWrite`. So a pusher can write fields it could never write through the
SPA.

The redaction bypass is a **symptom**. The disease is the parallel channel: any
read- or write-side rule (`visible:` redaction, field-write ACL, whatever comes
next) must be implemented, and kept in lockstep, in two places.

## Why the naive fix is wrong

Redacting `handleSyncGet` in place is a **data-destruction bug**. The sync
CLIENT round-trips a **whole record**: Pull does `GetEntity` →
`ApplyEntity(whole record)`, a whole-record REPLACE. A redacted GET body → the
stripped record lands in the replica's local store → a later Push replays the
erasure to the authoritative server. Hidden fields destroyed by a principal that
never saw them (the CLAUDE.md "never redact a read that feeds a write" rule). PR
#1329 did exactly this; a code review caught it.

An earlier "full unify" idea (sync writes through `/api/v1` too, retiring
`ApplyEntity`) is *also* wrong: it would require adding
automation-**suppression** to the shared SPA write core — but the SPA is a human
at the origin and MUST fire automations. See the investigation trail below.

## Fix: sync is a fancy browser

Reframe sync as **a client of the authorized API with no authority of its own**
— the remote is always authoritative; the replica optimistically holds local
state and reconciles to the remote, exactly like a browser rendering ahead of a
server round-trip. Three flows, each with a distinct rule:

**1. Read (pull-fetch) → `/api/v1` GET.** The replica fetches changed
entities/relations through the **same** read path the SPA uses. Row-gating AND
`visible:` redaction apply there, **once**. The field-redaction gap closes as a
*consequence* of there being one content channel — not a bolted-on step. Retires
`handleSyncGet`.

**2. Pull-apply → the replica's OWN local store, as a PATCH (like the
browser).** Applying changes that **already happened on the origin** — including
their automation effects, which arrive as their own feed events. The replica
applies each change **as a PATCH to its local store, exactly as a human editing
via the browser would**. So this path:
- runs **no automations** (idempotency: re-running double-applies and ping-pongs,
`apply.go` / RR-L1MY0N) — this is the replica applying a *log*, not re-deriving;
- **patches only the fields it is authorized to see**, using the `_redacted` wire
field (DEC-T0XIWQ, `redactedPropertyNames`) to disambiguate the three states of
a property: **present** in `properties` → visible current value → patch it;
**named in `_redacted`** → hidden from this principal → leave the local copy
untouched (its absence is *explained*, so it is NOT misread as a delete, and it
is never named on push); **in neither** → genuinely deleted on the origin →
unset it locally. This is what lets a redacted read faithfully drive a replica —
redacted ≠ deleted — with no per-field feed tombstones and no leak (names, not
values). A hidden field is out-of-scope for replication in *both* directions;
the replica isn't entitled to its value;
- uses the **remote's ETag** as its baseline for conflict detection (stores what
the remote returned, sends it back as If-Match) — the replica does NOT recompute
a local canonical hash and compare (the `/api/v1` `computeEntityETag` and
`canonical.HashEntity` are different token spaces — truncated + relation-folded
vs whole-record — so a locally-recomputed hash would never match). This is the
replica managing its own state — NOT a channel into anyone else's store, so it
needs no field-write ACL (there is no ACL on the replica's private store; the
remote enforces on push).

**3. Push (replica → remote) → the SPA `/api/v1` write API, UNCHANGED.** Local
changes go up through the same authorized write path a human uses:
- automations **fire** (a replica-originated change is genuine intent the origin
hasn't seen);
- `validateFieldWrite` field-write ACL **enforced** (closes the write leak);
- **push is a PATCH of visible fields only**, symmetric with pull. `_redacted`
names exactly the fields the replica must NOT send, so a redacted replica can
never erase the primary's hidden data (the primary merges the named fields onto
its raw record — same net effect as the SPA's merge-onto-raw). No whole-record
replace, ever;
- **ids are the remote's to mint.** The replica creates locally under a
**temporary id**, pushes a create **without dictating an id**, the remote mints
the real id and returns it, and the replica **renames its local doc** to match.
No id-preserving-create seam on `/api/v1`; the SPA API is used exactly as-is;
- **conflicts are surfaced to the USER to resolve**, not auto-reconciled. The
replica needs to *detect* a conflict (remote ETag moved / create rejected) and
hand it to the operator — it does not need the old sync channel's rich
409/412/422 taxonomy decoded into automatic retry logic.

**Schema handshake (config fetch) at sync start.** Because the replica now
addresses records through `/api/v1/{plural}/{id}`, it fetches the primary's
public config/metamodel ONCE per run to resolve `type → plural` (config is not
secret — root CLAUDE.md). This fetch doubles as a **schema-compatibility
check**: before syncing any record, the replica verifies the primary's schema is
compatible with its own (known types, compatible property shapes) and **fails
fast with a clear error** on divergence, rather than discovering it mid-splice
and risking silent corruption. Syncing two stores whose schemas have drifted is
a real corruption vector; the handshake closes it.

**Kept: the change feed / manifest.** `/api/sync/manifest` stays — content-free,
row-gated, cursor + tombstones. It is the one signal a plain GET can't express
(absence-from-a-list is not an explicit delete). The feed gate is **row-level
only**: cannot-read → omitted (pruned, indistinguishable from "no change");
can-read-but-only-some-props → still appears (the feed carries no prop values,
so partial-prop access is a fetch-time concern that never enters the feed).

**Retired:** `handleSyncGet`, `handleSyncPut`, and the sync-specific write-ACL
bypass. **Untouched:** the SPA `/api/v1` write core — no id mode, no suppression
flag, nothing. That is the whole point: the human write path keeps doing its one
job.

## Consequences / tradeoffs

- **N+1 fetches.** One feed poll → up to M authorized GETs vs. one fat batch
today. Accepted: parallelizable; a batch-GET can be added ON the authorized read
path later. Not a blocker.
- **Client rewrite is the bulk of the work.** The sync client (`internal/cli/sync`)
moves from `/api/sync/{entities,relations}` to `/api/v1`, and gains **temp-id →
remote-id reconciliation**: create locally with a provisional id, adopt the
remote id on push-ack, rename the local doc, and **remap references** — a
relation created locally as `TEMP→B` must be pushed with `from` resolved to the
remote id. Push ordering becomes explicit: create entities → adopt ids → push
relations with resolved ids.
- **Retires the parallel ACL, not just the leak.** `sync_handlers.go` stops
evaluating its own view of the world; only the row-level feed gate + cursor +
tombstone machinery remain.
- Covers entity fields and relation meta uniformly (the gap predates relations).

## Scope: Mode A only (CLI replica ↔ remote); Mode B deferred

There are two conceivable sync modes; **this ticket builds only Mode A.**

- **Mode A — replica ↔ remote (personal use), IN SCOPE.** The existing `rela
sync` CLI. The local side is an **fsstore with no ACL** (mobile app, CLI tools).
Redaction happens only on the *remote read* (what the primary sends this
principal); the local replica holds its own synced copy in the clear. So the
local pull-apply merge is **pure correctness, not a security boundary** — the
replica reads its own raw local record (no redaction locally), splices the
visible fetched fields on top **client-side**, and applies via the sanctioned
id-preserving, automation-suppressed `ApplyEntity`/`ApplyRelation`. No new
store-direct path; no `PatchEntity` needed at the local layer (that rule guards
a *redacted* read feeding a write — which does not occur locally here).

- **Mode B — server ↔ server, DEFERRED (separate ticket).** Purpose undecided.
Either redundant (geo-replication / HA is better served by postgres streaming
replication, byte-exact, no ACL semantics) or a **major undesigned feature**
(federation: org A shares a redacted slice with org B's server, B's users work
under B's own ACL). Federation raises unanswered questions — whose ACL governs
B's local copy, does B re-redact for its own users, id-space collisions between
two authoritative servers — that are out of scope here. The `_redacted` splice
built for Mode A IS the reusable primitive if federation is ever pursued, but
the wiring is a separate design. Do NOT assume the CLI design generalizes to a
running-server replica.

## Origin

TKT-B1F5Q1 review — RR-FOD7IB.

---

## Design trail (how we got to the fancy-browser model)

- **PR #1329 (redact the sync GET, keep both handlers)** — DRAFT/superseded. A
code review found it reintroduced the data-destruction bug (redacted body →
whole-record ApplyEntity → erasure). Its redaction work is subsumed: redaction
is inherited for free once sync reads via `/api/v1`.
- **"Full write-unify" (sync writes via `/api/v1`, delete ApplyEntity)** —
rejected. An investigation mapped the v1 write surface and found it was
*deliberately designed not to* preserve ids, suppress automations, or do
whole-record If-Match — exactly the guarantees `ApplyEntity` provides. "Retire
the sync handlers" that way would mean **adding an ApplyEntity-shaped mode to
the shared SPA write core** (a suppression seam on `updateCore`/`createCore`,
id-preserving create, If-Match on v1 delete/relations). The suppression seam is
actively wrong: the SPA is a human at the origin and MUST fire automations. This
is the opposite of the cleanup unification was meant to be.
- **Resolution (the fancy-browser split):** automation-suppression is a property
of the **pull-into-local** direction only (applying already-run changes), NOT of
"sync." Push is genuine origin-unseen intent → SPA API, automations fire.
Pull-apply is a local log replay → ApplyEntity, no automations. Ids are never
the replica's to choose — remote mints, replica renames locally. Net: the SPA
write core is UNTOUCHED, and the "second channel" is gone because push *is* the
SPA API and pull-apply is the replica's private state.
