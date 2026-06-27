---
id: TKT-POT9GQ
type: ticket
title: 'ACL read-side: SSE /api/v1/_events per-type gating — type-scoped staleness signal, ReadQuery-gated'
kind: enhancement
priority: high
effort: s
status: done
---

Final read-side leak after TKT-VQGN (per-entity) + TKT-VMD8 (list/sidebar) +
TKT-BA8BSX (search): the SSE feed `/api/v1/_events` broadcasts
`entity:created/updated/deleted` carrying `{type, id}` to **every** subscriber
with no ACL filtering — a `{type, id}` existence/timing oracle for entities a
principal cannot read.

## Design arc (recorded — the reasoning matters)

This ticket went through a long design exploration (2026-06-13) that is worth
preserving because it explains why the final design is *small*:

1. **Per-id gating** → blocked on the delete-can't-resolve problem (entity gone, relation-scoped verdict unrecoverable).
2. **Server delivered-set / mergebox** → unsound (RR-LBXIB2: stale verdicts never invalidated on relation/policy transitions).
3. **Opaque per-principal cacheId (HMAC)** → cryptographically sound and leak-free, BUT required a broker re-plumb, per-event gating (DoS), a deployment master secret with fail-loud distribution, and a load-bearing frontend cacheId map. Heavy.
4. **Use-case reframe (the unlock):** SSE here is NOT an event log or collaboration channel — it is a **cache-invalidation / staleness signal** for a *multi-writer, externally-editable store* (writes come from CLI/MCP, external file edits, `git pull`, automations, other processes, other tabs — not just this SPA). The client only needs to know *which active views to re-fetch*, and the re-fetch goes through already-gated endpoints. So the feed needs almost no precision.
5. **Granularity ladder:** the security cost is a CLIFF between per-type and per-id. Per-type gates on the *cheap* `ReadQuery(type)` verdict (no per-entity walk, no delete-leak, no cacheId). Per-id is the entire hard part. rela's writes are low-frequency (human edits, git pulls, automations — seconds apart), so per-type over-fetch is free in practice.

**Decision: per-type granularity.** Drop the id from the wire entirely. The
cacheId/mergebox/snapshot-ACL designs are captured as rejected alternatives (and
IDEA-CQMKMD for snapshot-ACL).

## The design (final)

**Wire payload: `{type}` only.** No id, no op (op does nothing the client acts
on — the re-fetch resolves create/update/delete naturally). The event means
"your active views of type T are stale; re-fetch them through the (gated)
endpoints."

**Gate: `readGate.ReadQuery(ctx, type)` per connection.** Withhold the `{type}`
event from a connection whose principal has `DenyAll` on that type.
AllowAll/Query → deliver. This is the CHEAP verdict — constant-time for
AllowAll/DenyAll, and even the Query case needs no per-entity store query here
(we're gating the TYPE, not an id). Resolve once per connection and cache;
refresh on the rare membership-changing events (member-of / role-relation writes
that pumpStoreEvents already sees) — RR-K2WKEJ staleness fix, now trivial
because it's a type-verdict not a per-entity walk.

**Debounce (always-on, orthogonal to granularity):** coalesce a burst of store
events into one `{type}` nudge per affected type per window (~100–250ms), both
server-side (a `git pull` landing 200 files → one nudge per type, not 200) and
client-side (debounce successive re-fetches). Kills the burst-amplification
concern.

**Broker plumbing (RR-GVHEIK, now significant-not-critical):** the principal IS
reachable — handleSSE holds r.Context() (attachACLRequest wraps the SSE route).
The only change: the broker carries the entity event's `{type}` (not a
pre-rendered `{type,id}` frame) so the per-connection handleSSE loop can apply
`ReadQuery(type)` before writing. Non-entity events (refresh, git:status) stay
on the existing path. Dedup/debounce can live in the per-connection loop or a
shared coalescer.

**Client (useEvents.ts):** on `{type}` event, invalidate active queries of that
type — which is essentially what it does TODAY
(`invalidateQueries(entityKeys.type(data.type))`). So the frontend barely
changes: just stop reading `data.id` (it's gone) and debounce. The cacheId map
idea is dropped entirely.

## Leak analysis (final)

- A principal receives a `{type}` nudge only for types they can read (`ReadQuery != DenyAll`). They learn "a type I can read had activity" — which they could already infer by polling the gated list endpoint's count. Residual ≈ per-type activity *timing* for readable types. Strictly less than the old per-id existence oracle.
- A `DenyAll` principal for a type receives NOTHING for it — no timing, no existence.
- No id on the wire → no delete-leak, no cascade-readability problem, no cacheId, no master secret. All retired.
- NopACL: every type nudge flows (ReadQuery → AllowAll). Near-byte-identical to today except the payload loses `id` (a SHRINK, not a new field) — the regression test asserts the client still invalidates correctly without the id.

## Out of scope

- Per-id / per-query precision (rung 3/4) — a future PERF optimization with a real motivating case (a hot high-write type), NOT correctness. cacheId design captured if per-id is ever needed.
- Collaboration / presence / per-keystroke (different feature; audit-isolation invariant forbids principal-topology on the feed anyway).
- Relation/attachment SSE events (not broadcast today).
- Snapshot-versioned ACL (IDEA-CQMKMD).
- MCP transport event streams (TKT-G3PPD).
- Soft-delete/trash/restore (its own feature ticket).

## Acceptance criteria

1. **Per-type gating.** Principal `read:[ticket]` on `/api/v1/_events`: receives a `{type:ticket}` nudge when any ticket is created/updated/deleted, receives NOTHING when a feature changes. Wire-byte assertion.
2. **Role-relation principals still get the type nudge.** alice/editor-of/PRJ-42 (Query verdict on ticket): receives `{type:ticket}` nudges (her verdict is not DenyAll). She does NOT receive `{type:feature}`.
3. **No id on the wire.** The SSE payload for an entity event is `{type}` only — no entity id/slug anywhere in the byte stream.
4. **DenyAll withholds.** A principal with no read grant on a type receives zero nudges for it (no timing signal).
5. **Debounce/coalesce.** A burst of N writes to one type within a window produces ONE `{type}` nudge per connection, not N. Asserted with a burst.
6. **Cheap gate + staleness (RR-K2WKEJ).** The type verdict is resolved per-connection and cached; a membership-changing event refreshes it (a principal removed from a conferring group stops receiving a type they lose). No per-entity walk on the hot path.
7. **Fail-closed on gate error (RR-MTUW2N).** A ReadQuery error drops the nudge, keeps the connection; never fail-open.
8. **NopACL.** Without acl.yaml, every type nudge flows; client invalidates correctly with the id-less payload.
9. **Audit-isolation invariant preserved.** TestSSE_DoesNotFlowAuditEvents green (the feed still carries no principal-topology; now it carries even less — just a type).
10. **Client.** useEvents.ts invalidates active queries by type on a `{type}` event; no id dependency; debounced. Existing reconnect re-fetch unchanged.
11. **Docs.** GUIDE-acl-security: SSE moves from "What still leaks" to gated; per-type design + the rejected per-id/cacheId alternatives + residual (per-type timing) documented; threat-model summary → "all read channels gated".

## Files

- `internal/dataentry/watcher.go` — broker carries `{type}` for entity events; handleSSE per-connection `ReadQuery(type)` gate (cached, membership-refreshed) + debounce; non-entity events unchanged
- `internal/dataentry/sse_acl_test.go` (new) — AC1–9
- `frontend/src/composables/useEvents.ts` — drop `data.id` use, debounce (mostly already type-based)
- `docs-project/entities/guides/GUIDE-acl-security.md` + regenerated `docs/acl-security.md`

## Stack

Stacks on PR 972 (`feat/acl-search-tkt-ba8bsx`) per user decision 2026-06-13.
Branch `feat/acl-sse-tkt-pot9gq`. 3-deep live stack (949 ← 972 ← this) accepted.
Effort now **s** (was l under the cacheId design) — the use-case reframe
collapsed it to "gate the existing type-feed on ReadQuery + debounce".
