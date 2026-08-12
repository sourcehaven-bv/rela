---
id: TKT-MF1CWZ
type: ticket
title: 'CalDAV: go-webdav adapter, VTODO collections under /api/, getctag + two-way PUT/DELETE'
kind: enhancement
priority: medium
effort: l
status: done
---

## Description

The CalDAV protocol surface itself: serve rela entities as `VTODO` collections
that Apple Reminders, Thunderbird, eM Client and Cfait can sync **two-way**.
Design in **RES-1Y2EB5** (Axes A, B, D).

Depends on **TKT-SNBQX0** (VTODO renderer), **TKT-UGYSC8** (mapping config),
**TKT-WAA092** (alias table), and the **targeted-write abstraction** (see
below).

### Mount under `/api/` — this is a security requirement, not a preference

`attachACLRequest` (router.go:213) and `requireVerifiedJWT` are **both** gated
on `isAPIPath`. A `/caldav/` prefix would get **no `acl.Request`, no read gate**
(`readGateFromContext` returns `nopReadGate{}` = permit-all, readgate.go:167)
**and no JWT gate** — which is also where the Pratique identity assertion
arrives. Mounting under `/api/v1/_caldav/…` inherits all of it.

Also avoid BUG-F3ADZO: a non-`/api/` route registered on the *inner* mux is
unreachable and silently returns the SPA's 200 HTML.

### Deliverables

1. **Vendor `github.com/emersion/go-webdav`** (MIT) with an arch-lint `vendors:`
grant **scoped to a thin adapter component**. Its `Backend` traffics in
`*ical.Calendar`; that type must **not** escape the adapter — `calfeed` stays
the domain model, per CLAUDE.md's "don't leak parsing types" rule.
2. **Implement the 10-method `caldav.Backend`**, sourcing entities through the
existing ACL-scoped `feedEntitySource` (feed_handler.go:143-179) — copy that
template verbatim; it already does the `ReadQuery` DenyAll/AllowAll/withhold
fail-closed switch.
3. **`getctag`** — go-webdav does not implement it (zero matches). Add it from
`calfeed.CollectionTag`. It **must be content-derived**: `store.Watcher` is
lossy by contract (store.go:830 — "if the subscriber's channel buffer is full,
events are dropped"), `pumpStoreEvents` drops the entity ID, and fsstore has no
monotonic sequence. A counter-derived ctag would silently skip changes.
4. **Multi-collection, VTODO-only.** Each `caldav:` config key is one
collection; `ListCalendars` / `CalendarHomeSetPath` enumerate them so the user
configures ONE account and sees every list. Each advertises
`supported-calendar-component-set` for its single component. Confirmed in the
live test: the client issues `PROPFIND Depth:1` over the home-set to discover
collections, Reminders bound to the VTODO-only one, and Calendar.app
`MKCALENDAR`'d a *separate* VEVENT one. Mixing components in one collection
breaks Reminders.
   - **Collection URLs must be STABLE and human-pasteable.** A changing href
re-adds every list as new. Use the config key (`/api/v1/_caldav/cal/<name>/`),
not a generated id — Thunderbird does not auto-discover collections at all and
needs a URL a human can paste.
   - **`MKCALENDAR` must be refused (405).** Collections are operator-declared
config; a client-minted one has no mapping and becomes an orphan. Radicale
accepted Calendar.app's `MKCALENDAR` in the live test and got exactly that.
5. **Inbound `PUT`** — apply the mapping, resolve identity via the alias table,
create or update through `entitymanager.Manager`.
6. **`DELETE` → configured status transition** (`on_delete:`), **not** a
destructive delete. rela has no soft-delete and `DeleteEntity` cascades to
relations. Real delete stays opt-in; 403 is the fallback when unconfigured.

### Hard dependency: the targeted-write abstraction

A CalDAV `PUT` carries a **partial** view of an entity — Reminders sends
`SUMMARY`, `STATUS`, `COMPLETED`, `PERCENT-COMPLETE`, `DUE` and drops everything
it does not model. Applying that as a whole-entity save would **erase every
property VTODO has no slot for**, including all redacted ones.

**Do not implement inbound writes on the raw store handle.** Brief:
`.ignored/prep-targeted-write-abstraction.md`.

### Deferred: sync-collection

go-webdav's `handleReport` dispatches only `calendar-query` and
`calendar-multiget` (else 400). Per sabre/dav's client guide, `sync-collection`
is **optional** — clients check `supported-report-set` and fall back to
ctag/ETag polling, so the gap costs efficiency, not correctness. Design the
ctag/ETag layer so it can be added without rework. Radicale's
`storage/multifilesystem/sync.py` is the reference: the token is a content hash
over present **and past** item etags (deleted hrefs included = tombstones), with
per-token state snapshots and an error on unknown token forcing full re-sync.

### Acceptance criteria

1. Apple Reminders adds the account and sees EVERY configured VTODO collection
as its own list, from one account URL.
2. Checking a todo off in Reminders writes back — the entity's status changes
and **no other property is lost**, including properties the caller cannot read.
3. Creating a todo in Reminders creates an entity of the collection's
`entity_type` (one type per collection, so no attribution ambiguity).
4. Deleting a todo applies the configured status transition, not a delete.
5. A conflicting `PUT` (stale `If-Match`) returns 412.
6. `getctag` changes when any member changes and is stable otherwise.
7. All reads are ACL-scoped; an entity the principal cannot read is absent from
the collection (not 403 — indistinguishable from nonexistent).
8. `*ical.Calendar` appears nowhere outside the adapter package;
`just arch-lint` passes.
9. Verified against **Reminders (macOS + iOS)**, **Thunderbird**, **eM Client**,
**Cfait**.

### Sync cost model, and why the ctag is the hot path

CalDAV has three tiers of "avoid fetching everything", and they compose:

1. **`getctag`** (calendarserver extension, universally supported) —
`PROPFIND Depth:0` for one property. Unchanged ⇒ the client stops. This runs on
EVERY poll.
2. **ETag listing** — `PROPFIND Depth:1` for `getetag`, one line per resource;
the client diffs and fetches only changed bodies via `calendar-multiget`. O(n)
lines and O(n) ETag computations.
3. **`sync-collection`** (RFC 6578, deferred above) — token in, delta out
(changed resources + `404` tombstones + a new token). O(changes), not O(n).

**The ctag is the highest-value optimization, not `sync-collection`.** Tier 1
runs on every poll whether or not anything changed, so if the ctag is expensive
the cursor saves nothing on the common path. Today it IS expensive:
`calfeed.CollectionTag` calls `TodoETag` per entry, and `TodoETag` calls
`RenderTodo` — so computing a ctag **re-renders the entire feed**.

Measured on the current renderer (100 / 1k / 10k todos):

|                          | 100     | 1 000   | 10 000  |
| ------------------------ | ------- | ------- | ------- |
| `RenderCollection`       | 0.42 ms | 1.76 ms | 13.3 ms |
| `CollectionTag`          | 0.13 ms | 1.34 ms | 13.7 ms |
| allocs (`CollectionTag`) | 2 104   | 21 006  | 210 024 |

Everything is linear — O(n·m) in total content size, nothing quadratic — but
`CollectionTag` allocates MORE than a full render while returning 32 bytes.

**Design the ctag so it need not re-render.** Options, in rough order of appeal
— decide during planning, and note this may want its own ticket:

- **Cache the ctag, invalidate on the store event feed.** `pumpStoreEvents`
already broadcasts a type-scoped change signal. Caveat: `store.Watcher` drops
events when a subscriber's buffer fills, so a dropped event would strand a
client on a stale ctag — needs a safety re-computation interval or a watermark
cross-check.
- **Aggregate a cheaper per-entry fingerprint** than a full render (hash the
mapped property values before serialization). Changes the ctag's derivation, so
it must still change iff rendered content changes — an entity edit invisible to
the mapping must NOT move it.
- **postgres only:** `rela_seq` gives a monotonic per-row watermark, so
`max(seq)` over the collection is a natural ctag. But fsstore has only
wall-clock `UpdatedAt` with no sequence, so this cannot be the only mechanism.
- **Render-once accessor** returning `(bytes, etag)` per entry, so the Depth:1
listing path does not pay the render twice. Helps tier 2, not tier 1.

Whatever is chosen must keep the **content-derived** property: store events are
droppable and fsstore has no sequence, so a counter-derived ctag would silently
skip a change (see `CollectionTag`'s doc).

Also note the upstream bound, which dominates all of the above:
`declarativeFeed.List` loads and ACL-gates every candidate entity per request
with no pagination or cache. Rendering 10k todos costs ~13 ms; loading and
ACL-scoping 10k entities costs considerably more.

### Chosen direction for postgres: seq-watermark sync-token (DECIDED)

**The postgres backend already has everything RFC 6578 needs.** This was
discovered after the cost model above was written and it changes the plan:
`sync-collection` moves from "hard, deferred indefinitely" to "the natural
implementation on postgres".

What exists today (migration 0001 + 0003):

- **`rela_seq`** — a global monotonic sequence bumped on EVERY mutation, not
just inserts: `UPDATE ... seq = nextval('rela_seq')` appears on entity update,
rename, relation update, and attachment write.
- **`entities_seq_idx` / `deletions_seq_idx`** — so `WHERE seq > $1` is an index
range scan, not a seqscan.
- **`deletions`** — hard-delete tombstones carrying their own seq, written in
the SAME transaction as the delete. **This is the tombstone problem solved.**
The Radicale-style per-token state snapshots (hash over present AND past item
etags) are unnecessary here.
- **`pgstore.ManifestSince`** — already implements "live rows with seq > X UNION
tombstones with seq > X, ORDER BY seq" for the sync feature.

**Design:**

- **sync-token** = `data:,<seq>` — the client's watermark, opaque to it.
- **delta query** = type-scoped manifest: live entities of the collection's
`entity_type` with `seq > cursor`, UNION tombstones with `kind='e'` and matching
`typ`. True **O(changes)**, index-backed.
- **`getctag`** = `GREATEST(max(entities.seq), max(deletions.seq))` scoped to
the type. Must include deletions: a delete removes a live row, so `max` over
live rows alone can go DOWN. Index-only, sub-millisecond regardless of row count
— replacing today's O(n·m) full re-render.
- **Over-triggering is accepted and expected.** Both the ctag and the delta are
scoped by entity TYPE, not by the collection's `where:` filter, so an entity the
filter excludes still moves the tag. That is the SAFE failure direction (a
spurious re-sync is self-correcting; a missed change strands a client forever),
feed content is mostly static, and clients poll on the order of 15 minutes. Do
not add filter-awareness to chase this.

**Per-backend, following the `HistoryReader`/`VersionService` precedent:** an
optional capability that postgres implements and other backends do not.
fsstore/memstore fall back to the content-derived `calfeed.CollectionTag` and
simply do not advertise `sync-collection` in `supported-report-set` — clients
then degrade to ctag+ETag polling, which is exactly what the RFC expects. Local
single-user deployments do not need the optimization.

**Open items for planning:**

- `ManifestSince` is NOT directly reusable: it is unfiltered by type,
unpaginated, and returns the whole result in one slice. CalDAV needs a
type-scoped, ACL-gated, ideally paginated variant. Decide whether to generalize
it or add a sibling.
- **ACL interaction is NOT a blocker** — an earlier note here claimed a
per-principal watermark would be self-defeating because it needs an ACL-filtered
`max(seq)`. That was wrong: `internal/visibility/pushdown.go` already composes
the ROW gate as a store predicate (`acl.ReadQueryResult` is `AllowAll` /
`DenyAll` / a `GraphQuery`), so a per-user watermark is `max(seq) WHERE type=$1
AND <pushdown predicate>` — one index scan, and free outright in the `AllowAll`
case that most deployments hit. Ship the GLOBAL watermark first anyway:
over-triggering costs one wasted listing, clients poll on independent timers (no
synchronized stampede — a write disperses roughly uniformly across the poll
interval), and the waste is bounded by WRITE rate, not user count. The per-user
variant is a drop-in later, being the same query plus a predicate. Residual
caveat: pushdown replaces the row gate only — field-level `visible:` redaction
is not expressible as a predicate, so a redaction change would not move a
row-scoped watermark. Low risk (`visible:` rules change with config, not
per-write), but do not assume it away.
- If several collections project the SAME entity type they share a ctag and
cross-trigger. Acceptable per the over-triggering note, but confirm against the
final config shape.
- **Tombstone retention is currently unbounded** (nothing prunes `deletions`),
which conveniently means NO sync-token can expire. If pruning is ever added (a
documented follow-up on the sync feature), CalDAV must then return `507` for a
token older than the retention horizon and force a full re-sync.

### Inherited from the TKT-SNBQX0 review

- **The real AC6 round trip lands here.** `calfeed` is render-only, so the
parse-and-re-render check against the captured Apple fixtures belongs in this
adapter package, where go-ical already lives. Import
`internal/calfeed/testdata/*.ics` rather than re-capturing.
- **Pre-existing VEVENT-path defects to fix while adjacent** (deliberately out
of scope for the renderer ticket, since they need the VEVENT fixtures and
callers re-verified): `RRULE` is written by raw concatenation like `STATUS`/
`TRIGGER` were (ical.go:96), and `ETag`/`Event` has no per-field sensitivity
test.
- **The Component/slice mismatch is not an error at render time** — a
`Feed{Component: ComponentTodo, Events: [...]}` silently renders empty. The
config-load validation in TKT-UGYSC8 is the right place to reject it; make sure
that check exists.

### Conformance risk

Vikunja's CalDAV self-describes as "early alpha, has bugs" — budget real
per-client testing. Thunderbird has reported quirks about tasks landing only in
"Inbox". The `.ignored/radicale-test/` rig is a working reference server to diff
rela's output against.

## Deletion semantics: the alias table is the tombstone (implemented)

A client that has not synced will PUT its cached copy of a to-do deleted in
rela. Treating that as a create resurrects a deliberate deletion, so it is
refused with **404** (not 409 — a CalDAV client reads 409 as "retry later" and
re-sends forever, while 404 tells it to drop its local copy).

The rule is an inference from server-side state alone:

> alias exists + entity missing => it was deleted after we served it => 404

The alias is RETAINED on the refusal; dropping it would discard the evidence,
and the next PUT would read as a create and resurrect the entity.

**Why not a marker property.** An earlier design stamped each served VTODO with
`X-RELA-ENTITY-ID` and treated its presence as proof. RFC 5545 3.8.8.2 says user
agents "can ignore" x-properties, so a client that strips it makes a stale write
look new — a heuristic that fails OPEN. Reverted.

**Why not a store-level tombstone table.** postgres already has one
(`deletions`, written in the same tx as the DELETE) but the filesystem backend
cannot have an equivalent: `rm` / `git pull` / an edit while the server is
stopped fires no hook and no event, and fsstore's startup `syncEntities` adopts
whatever is on disk without diffing the previous index. Asking "does it exist
now?" needs no deletion record, so one code path covers both backends.

Verified live against the demo, including a genuinely `rm`-ed entity:

| PUT | Result |
|---|---|
| alias present, entity deleted (incl. out-of-band `rm`) | 404, stable across retries |
| no alias, unseen href | 201, created |
| create -> out-of-band delete -> replay cached PUT | 404 |

**Follow-up:** retained aliases grow monotonically as entities are deleted.
Noise at PIM scale; a multi-user postgres deployment wants pruning (a deletion
timestamp on the alias, or reconciliation against `deletions`).
