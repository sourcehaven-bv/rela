---
id: TKT-9TOEBH
type: ticket
title: 'pgstore: one shared NOTIFY channel with schema in the payload, not one channel per schema'
kind: refactor
priority: medium
effort: m
status: backlog
---

## Description

The cross-process change feed names its NOTIFY channel `rela_changed_<schema>`
(`feed.go:42,65-74`). Because `LISTEN` requires a dedicated session per
connection, a process serving N schemas needs **N long-lived connections just to
hear about changes** — before any pooling for actual queries.

Replace the per-schema channel with **one channel (`rela_changed`) carrying the
schema as a payload field**, and have the listener filter. Live updates then
cost **one connection per process** instead of one per schema.

### Why (stands on its own)

Today `pgstore.Open` opens a dedicated `pgx.Connect` purely to `LISTEN`
(`listener.go:72`), outside the pool. That is one non-pooled connection per
store, permanently held. Even in the current single-schema deployment this is a
fixed cost per process; the multi-process deployment story in
`docs/postgres-backend.md` already notes "each process opens one extra
connection to receive change notifications".

The per-schema name buys **nothing that the payload cannot**: the listener
already parses a structured payload and already filters (it drops self-echoes by
`originID`, `listener.go:200-202`). Adding a schema field and one more filter
predicate is strictly less machinery than a dynamically-named channel that must
be resolved at connect time and kept in sync with the producer (`listener.go:82`
mutates `s.channel` as a side effect — a wart this removes).

### Why it also matters later

Multi-tenant SaaS (RES-D54281) is capped by exactly this. The connection budget
is ~17/tenant today, and the analysis there identified the LISTEN session — not
the pool — as the term that does **not** shrink under apartment-style pooling.
This is the change that decouples "live updates" from "tenants per process".

## The important subtlety: LISTEN is shareable, catch-up is not

Do not conflate the two halves. Verified:

- The **dedicated connection is used ONLY for `LISTEN`** (`listener.go:72-96`).
- **`primeWatermark` (`listener.go:224`) and `catchUp` (`listener.go:254`) run on
`l.store.db` — the store's pool**, not the dedicated connection.

That matters because the catch-up query is **unqualified SQL** (`FROM entities`,
`FROM relations`, `FROM deletions`) resolved through the connection's
`search_path`, and the watermark is a `rela_seq` value that is **per-schema**. A
single shared connection therefore *cannot* run catch-up for N schemas, and a
shared watermark would be meaningless.

So the split is:

- **Shared**: the LISTEN session and the channel. One per process.
- **Per-schema, unchanged**: the watermark, `primeWatermark`, `catchUp`, and the
30s safety-net poll — each already bound to its own store's pool.

Getting this wrong in the direction of "share everything" silently breaks
recovery of missed notifications, which is precisely the failure mode the
watermark exists to prevent — and it fails *quietly*, like the sweep-lock bug in
RES-S8CH9C's R1.

## Scope

**In scope**

- Change `feedChannelPrefix` usage to a single constant channel name; drop
`resolveChannel`'s per-schema suffix (keep resolving `current_schema()` — it
becomes a payload field instead of part of the name).
- Add `schema` to the payload (`notifyPayload`/`parseFeedPayload`,
`feed.go:95-130`); bump `payloadFields` 7 → 8.
- Filter in `handleNotification`: ignore a notification whose schema is not this
listener's, alongside the existing `originID` self-echo check.
- Remove the `s.channel` side-effect assignment in `startListener`
(`listener.go:82`) — with a constant channel, producer and consumer no longer
need to be reconciled at runtime.

**Out of scope**

- Sharing ONE listener goroutine/connection across N stores. This ticket makes
the channel shareable; actually multiplexing one connection to many stores is
the follow-up, and depends on the `appbuild.Services` split (RES-D54281).
**Landing this first means that follow-up is wiring, not protocol design.**
- Redis or any external bus. That is the cross-cluster answer (RES-D54281), not
this.
- Anything touching the watermark, catch-up, or the sweep.

## Compatibility: this is a wire-format break

An 8-field payload fails the 7-field parse in an old listener, and vice versa.
`parseFeedPayload` returns `ok=false` on a bad payload and the listener
**degrades to catch-up** (`feed.go:110-113` documents exactly this) rather than
corrupting — so a mixed-version fleet stays *correct*, but loses live push until
every process is upgraded, self-healing within `catchUpInterval` (30s).

That is an acceptable, bounded degradation, but it MUST be stated in the PR and
release notes: **during a rolling upgrade, cross-process live updates lag by up
to 30s.** Old and new processes also LISTEN on different channel names, so they
simply do not see each other until both sides are new.

## Acceptance criteria

1. All processes LISTEN on one constant channel regardless of schema.
2. A notification for schema A is ignored by a listener bound to schema B, pinned
by a test using two schemas on one database (the harness already does this —
`listener_test.go:53` returns the schema name for exactly this purpose).
3. Self-echo filtering by `originID` still works, unchanged.
4. Two stores on two schemas in one process both receive their own live events
and neither receives the other's.
5. A malformed/short payload still degrades to catch-up rather than erroring or
emitting a wrong event.
6. `primeWatermark`/`catchUp` behaviour is untouched — same tables, same overlap,
same per-schema seq semantics.
7. `s.channel` is no longer assigned from the listener.
8. Fail-before check: each new test must be shown to fail against the pre-change
code. A schema-filter test is exactly the shape that can pass vacuously (see
RES-S8CH9C R1's note that the obvious lock-scoping test passed against a
deliberately broken build).
9. `just test-postgres` passes; `just ci` passes.

## Notes

- `notifyDisabled` (`feed.go:19`) and `catchUpInterval` (`listener.go:44`) are
package-level test hooks. They stay process-global here; with N stores per
process they would become a shared knob, which is fine for tests but worth a
comment.
- The payload separator is `\x1f`, rejected by `storeutil.ValidateID` in ids and
relation types (`feed.go:44-47`). **Schema names must be validated the same
way** before landing in the payload — an unvalidated schema containing `\x1f`
would shift every field. Schema names are operator/tenant-derived, so this is a
trust boundary, not a formality (RES-D54281 already requires
`^[a-z][a-z0-9_]{0,30}$` for tenant schema names).
- `docs/postgres-backend.md:103-110` documents the per-schema channel and the
"one extra connection" cost; update both.
