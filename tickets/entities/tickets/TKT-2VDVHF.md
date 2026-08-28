---
id: TKT-2VDVHF
type: ticket
title: 'Autosave conflict resolution: per-field version preconditions, three-way merge on 412, bounded auto-retry'
kind: enhancement
priority: high
effort: l
status: backlog
---

## Problem

Two users — or one user in two tabs — editing the same entity silently lose one
edit. Optimistic locking is **fully implemented server-side and entirely unused
client-side**.

### Confirmed by investigation (re-verified 2026-08-28 against current develop)

1. **No client path sends `If-Match`, and none currently can.** The `etag`
parameter threads through `client.ts:42-49` → `api/entities.ts:181-185` →
`stores/entities.ts:159-162`, but **the ETag is never captured**: every axios
method unwraps to `response.data` (`client.ts:34, 39, 47`), discarding headers.
`EntityCache.etag` was declared, written nowhere, read nowhere — a dead field,
now deleted (TKT-52OFC9). The server's ETag (`api_v1.go:772` on GET,
`write_handler.go` on PATCH) is dropped at the client boundary.
   - Callers verified passing NO etag: the three autosave channels
(`useAutoSave.ts:380, 449, 495` — literal `undefined`), explicit save
(`DynamicForm.vue`), Kanban drag-drop, list bulk-set. Only `relaBridge.ts:95`
forwards one, and only if a hosted app supplies it.

2. **The documented conflict strategy does not exist.** `useAutoSave.ts:18-20`
claims *"cross-tab conflicts resolve through the SSE merge path."* SSE
`entity:changed` carries `{"type": "..."}` ONLY — no id, no payload
(`watcher.go`), deliberately, to avoid a per-entity existence oracle
(TKT-POT9GQ). No edit-form component subscribes. `mergeServerResponse` is
called only from useAutoSave's own PATCH-response handlers, so it never sees a
server-pushed change. **Cross-tab edits are silently lost too.**
   - The comment's FIRST clause ("the FIFO chain already serializes per
composable instance") is **true and load-bearing** — see RR-DBL90Y. Only the
second clause is false. Correct the comment; do not discard the rationale.

3. **Failure mode today.** `mergeServerResponse` applies server values per
property but skips locally-dirty fields (`if (k in pending) continue`,
`useAutoSave.ts:596-597`). Alice and Bob both edit one field → Bob's dirty field
ignores Alice's value → Bob's next debounced PATCH overwrites her. No 412 (no
header sent), no conflict UI, and — because the version sweep debounces on idle
and Bob's write reset the timer — often no version row capturing Alice's text
either. **Silent last-write-wins.**

Different-field concurrent edits merge correctly today and MUST keep doing so.

## Root cause of the design change

The original plan sent a whole-entity `If-Match`. Design review produced five
critical/significant findings that all trace to **one mismatch**:

> **The ETag hashes the WHOLE entity, but the wire protocol is a SPARSE PATCH.**

`computeEntityETag` (`api_v1.go:2001-2034`) folds id + type + content + ALL
properties + ALL outgoing edges into one sha256, truncated to 8 bytes. PATCH,
meanwhile, is field-granular (`write_handler.go:386-390`: `Properties`,
`PropertiesUnset`, `Content`, `Relations`) and does a server-side
read-modify-write (`maps.Copy` over the raw entity, `:395`/`:411`/`:445-450`),
so **absent keys survive**.

So a whole-entity precondition rejects a write over fields the writer never
touched and often cannot even see. Every workaround for that in the original
plan was a retreat toward last-write-wins.

## Approach: per-field version preconditions

The server exposes a **version token per property**, plus one for content and
one for the relation set. A PATCH carries preconditions **only for the fields it
writes**. A 412 fires only when a *contested* field changed.

This **supplements** the whole-entity ETag; it does not replace it. The existing
ETag stays exactly as-is for HTTP caching (`If-None-Match`/304), where
whole-entity semantics are correct.

### Why this design (it dissolves findings rather than working around them)

- **RR-X52UBP** (critical — hidden-field churn made the entity permanently
  unwritable): a precondition on `title` is blind to a Lua task writing
  `salary`. No 412, no retry loop, no unresolvable conflict UI. The finding's own
  proposed fix — falling back to PATCHing *without* `If-Match` — was a retreat to
  last-write-wins, i.e. the bug this ticket exists to fix.
- **RR-R2A2T5** (critical — ACL-redacted properties erased): preconditions are
  built from the keys the client is actually **writing**, so a redacted field can
  never enter the precondition set and can never be unset by the merge. The
  invariant becomes structural rather than a rule to remember. Today this is safe
  only by accident (server-side `maps.Copy`).
- **RR-DBL90Y** (significant — three channels share one ETag, self-inflicted
  412s): disjoint precondition sets do not collide. A content PATCH and a
  property PATCH no longer invalidate each other.
- **RR-VQQQ60** (critical): still needs the DynamicForm merge-base fix (shipped
  separately as TKT-52OFC9), but an absent base now degrades to a single-field
  conflict rather than a whole-document one.

### Cost: verified acceptable

The concern was that per-field tokens make every GET more expensive. They do
not.

- **It is the same fold, not collapsed.** `computeEntityETag` already sorts
  property keys and writes `k=%v;` per property. A per-field token is that same
  per-key write, hashed per key instead of accumulated into one digest. Same
  number of `fmt.Fprintf` calls; N small hashes instead of one large one.
- **Relations cost nothing extra.** The worry was that folding
  `outgoingRelations` per-field would multiply store round-trips. It does not,
  for two reasons: (a) relations get **one** token for the whole edge set, not
  one per edge — the autosave relations channel writes the set as a unit, so
  per-edge granularity buys nothing; and (b) **the GET handler already calls
  `outgoingRelations` twice** — once at `api_v1.go:764` for serialization and
  once inside `computeEntityETag` at `:772`. Threading the already-fetched slice
  into the token computation removes a redundant store query, so this path gets
  *cheaper*, not more expensive.
- **No storage schema change.** Tokens start as derived hashes computed from
  data already in hand. No new column, no migration, nothing to backfill. If
  contention ever warrants stored per-field version counters, that is a later
  optimization behind an unchanged wire shape.

### Wire shape

**GET** `/api/v1/{plural}/{id}` gains a sibling to `_fields` / `_relations`:

```json
{
  "id": "TKT-001", "type": "ticket",
  "properties": {"title": "Fix login", "status": "open"},
  "_versions": {
    "properties": {"title": "a3f1c802", "status": "9b21de44"},
    "content": "77c0a19e",
    "relations": "e40b1a3d"
  }
}
```

Each token is the first 4 bytes of `sha256(entityID | type | fieldKey | value)`,
hex-encoded — the same per-key material the ETag already folds, salted with the
key so two fields holding the same value get different tokens. Only
**non-redacted** keys appear: `_versions.properties` is built from the same
post-redaction map the wire entity carries, so a client cannot learn that a
hidden field exists, let alone that it changed. The whole-entity `ETag` header is
unchanged.

**PATCH** accepts an optional `preconditions` object, parallel in shape to the
write itself:

```json
{
  "properties": {"title": "Fix login flow"},
  "preconditions": {"properties": {"title": "a3f1c802"}}
}
```

Rules, all of which are refusals rather than best-effort behaviour:

- A precondition key **not** present in `properties` / `properties_unset` is a
  **400**, not a silently-ignored hint. Accepting it would let a client
  precondition on a field it is not writing — reintroducing whole-entity
  semantics through the back door, and (for a field it cannot read) turning the
  endpoint into a change-detection oracle for redacted data.
- Preconditions are **optional and per-field**. Omitting them entirely is
  today's behaviour, so MCP, CLI, Lua and any other client are unaffected.
  Making preconditions mandatory is explicitly out of scope.
- `content` and `relations` preconditions are scalars, checked only when the
  PATCH writes content / relations respectively.
- Checking happens where the `If-Match` check lives now
  (`write_handler.go:375-384`), against the same ungated `h.reader.getEntity`
  seam, before any mutation. `If-Match` and `preconditions` compose: both are
  checked, either can 412.

**412 response** names the losers, so the client knows what to merge:

```json
{
  "errors": [{
    "code": "precondition_failed",
    "title": "Entity has been modified",
    "detail": "2 fields changed since your last read",
    "meta": {
      "conflicts": {
        "properties": {
          "title":  {"expected": "a3f1c802", "actual": "5d77f0b1"},
          "status": {"expected": "9b21de44", "actual": "1c04ba9f"}
        }
      },
      "versions": { "...": "current _versions block, so the retry needs no refetch" }
    }
  }]
}
```

Carrying `versions` in the error body is what lets the retry proceed **without a
GET**, which is how RR-PP9UEF's stale-cache trap is avoided structurally rather
than by remembering to pass `force=true`.

### API versioning cost

None. `_versions` is an additive response field (clients ignore unknown keys),
`preconditions` is an optional request field, and no existing behaviour changes
when it is absent. No `/api/v2`, no negotiated header, no deprecation window.

## Client work

1. **Capture `_versions` on the read path.** It rides in the JSON body, not a
   header — so unlike the original plan, **no axios envelope refactor is needed**.
   `client.ts`'s `.data` unwrap stays as it is.
2. **Autosave sends preconditions for exactly the keys it writes.** The three
   channels each build their own precondition set from their own body, so they
   cannot collide (RR-DBL90Y). The retained versions map is a **single mutable
   ref updated inside the then-handler** from every PATCH response — never
   captured at enqueue time.
3. **On 412: merge the named conflicting fields → re-PATCH** with the fresh
   tokens from the error body. Per field: base = `lastSeenServer[k]`, ours = the
   attempted value, theirs = the server value.
   - `theirs === base` → only we changed it → keep ours
   - `ours === base` → only they changed it → take theirs
   - both differ and differ from each other → genuine conflict
4. **Bounded auto-retry: 3 attempts, jittered backoff.** Surface UI only after
   the bound is exhausted or a genuine conflict is found.
5. **Markdown body: real diff3 text merge.** Base is `lastSeenContent`; content
   is line-oriented so disjoint hunks auto-merge. Use an existing library.
   **Never write git-style conflict markers into the entity** — in git a
   developer resolves markers in a working tree; here they would persist to the
   entity, land in `entity_versions` (append-only; the audited purge path
   refuses while a live row holds the content) and render in the SPA.

## Merge domain (RR-P6ZFSV, RR-R2A2T5 — specified, not left to the implementer)

- The merge domain is **the keys named in the 412's `conflicts` block** — which
  by construction is a subset of the keys we are writing.
- A key the patch omits is **UNCHANGED**, never absent-and-therefore-deleted. An
  automation-managed field (`updated_at`, `{{today}}`, status transitions —
  `manager.go` runs automations that mutate the entity before persisting) is
  therefore *incapable* of conflicting: we never precondition on it, so it never
  enters the domain.
- **`properties_unset` is NEVER emitted from theirs-absence.** Deletion is
  expressible only via the local UNSET sentinel — an explicit user act. A
  redacted property is simply absent from the wire
  (`affordances.go` `stripHiddenProperties` deletes the key; the `Inaccessible`
  marker is populated only for git-crypt-locked entities), so absence must never
  be read as intent to delete. The original plan's edge case ("deletion-vs-edit
  is a genuine conflict") was **stated backwards** and is corrected here.

## Control-flow invariant (RR-U68IVA)

**A merge result carrying ANY conflict entry MUST NOT be written.** Re-PATCH
only on a fully clean merge. This is a control-flow rule, not a property of the
merged output: `node-diff3`'s `mergeDiff3` returns `{conflict: true, result:
[...]}` with marker strings inline by default, so an implementer who wires
`merged` into `patch.content` and *separately* raises the UI ships markers into
the entity. Tests assert **no PATCH is issued** on a conflicting body merge, in
addition to asserting the output is marker-free.

## Decisions taken here

- **`dirtyFormRegistry` is DELETED, not wired** (RR-QSO6HF). `anyFormDirty`
  answers "is ANY registered form dirty for this property", unioning across
  every form registered for the entity. The merge needs *this* instance's dirty
  state, which `useAutoSave` already has precisely (`isDirty`, and
  `mergeServerResponse`'s `k in pending` / `k in timers` checks). Wiring the
  union in would let a side panel's dirty `status` preserve the *main* form's
  stale `status`, clobbering the server value from a form the user is not
  editing. `anyFormDirty` has **zero production consumers** (only
  `dirtyFormRegistry.test.ts`); `registerForm` is called from `DynamicForm` but
  feeds nothing. The SSE-refetch path it was built for does not exist.
  Deleting removes `dirtyFormRegistry.ts`, its test, and the `DynamicForm`
  registration.
- **The explicit DynamicForm save is narrowed to a dirty-field delta before it
  sends preconditions** (RR-U3ZF9A). It currently sends the entire `formData` —
  every property loaded at `loadEntity`, not a delta. Preconditioning a full
  hour-old snapshot would either conflict on fields the user never touched or,
  resolved toward ours, overwrite a concurrent edit with stale values. Sending
  preconditions only for genuinely-changed fields also keeps the write itself
  smaller.
- **`baseRecorded` sentinel** (RR-VQQQ60). `undefined` is ambiguous between
  "never seen the server" and "genuinely absent server-side". The merge
  **refuses to run and falls back to current behaviour** when no base was
  recorded. TKT-52OFC9 already seeds the base in `DynamicForm`; the sentinel is
  the guard that keeps the merge honest if a future surface forgets.
- **The 412 retry never reads through the TTL cache** (RR-PP9UEF). The error
  body carries current tokens, so the common path needs no refetch at all. Where
  a refetch *is* needed, it must bypass the 60s TTL — a cached entity has no
  meaningful version tokens, and `update()` rewrites that cache entry on every
  PATCH, so during an active autosave session the entry is continuously renewed
  and effectively always "valid". Long-term this dissolves into FEAT-XY2D1L (see
  below).

## Out of scope

- **CRDT / concurrent collaborative editing.** User was explicit: simultaneous
  edit is not the common case.
- **Mandatory preconditions server-side.** Breaks every other client and 412s
  the disjoint-field case the design already handles.
- **Stored per-field version counters.** Derived hashes first; storage only if
  measurement demands it.
- **Reviving SSE-into-open-form** (payload is type-only by security design).
- **Migrating `DynamicForm` to the Pinia Colada query layer** — FEAT-XY2D1L. This
  ticket is NOT blocked on it.

## Alternative weighed

**Temporary edit lock** — a short-lived advisory lock when a form opens, so the
second user is told up front rather than at save time. Stronger UX guarantee (no
lost work at all) but much heavier: needs a server-side mechanism, a lock-holder
identity, a TTL and a steal path for stale locks (crashed tab, closed laptop).
Not mutually exclusive — a lock could be layered later for high-contention
types. Per-field preconditions reduce the contention that would motivate it.

## Relationship to other tickets

- **TKT-52OFC9** — the DynamicForm merge base + dead `EntityCache.etag`. Shipped
  independently; a prerequisite for the merge, valuable on its own.
- **FEAT-XY2D1L** (Pinia Colada query cache) — the strategic home for the cache
  problem behind RR-PP9UEF. `frontend/src/queries/entities.ts` already has
  hierarchical keys (`['entities', type, 'detail', id]`) that SSE invalidates by
  prefix, and its own comment says it "replaces the entities-store TTL cache view
  by view". `fetchEntity` has only TWO consumers outside the store
  (`DynamicForm.vue`, `HistoryView.vue`). Near-term this ticket bypasses the TTL
  on the version-bearing path; long-term the migration dissolves the problem.
  **Not a blocker.**
- **TKT-0IGI4V** (flush-on-author-change) — complementary, not an alternative:
  this ticket PREVENTS the silent overwrite; the flush makes the pre-overwrite
  state recoverable in version history if one happens anyway.
