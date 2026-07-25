---
id: TKT-2VDVHF
type: ticket
title: 'Autosave conflict resolution: send If-Match, three-way merge on 412, bounded auto-retry'
kind: enhancement
priority: high
effort: m
status: planning
---

## Problem

Two users editing the same entity silently lose one's work. **Investigation
2026-07-25 found the gap is wider than first described.**

### Confirmed by investigation (file:line evidence)

1. **NO client path sends `If-Match` — and none currently can.** The `etag`
parameter threads through `client.ts:42-49` → `entities.ts:175-186` →
`stores/entities.ts:155-172`, but **the ETag is never captured**: every axios
method unwraps to `response.data` (`client.ts:33-34, 38-39, 47-48`), discarding
headers. `EntityCache.etag` (`stores/entities.ts:15-19`) is declared, written
nowhere, read nowhere — a dead field. The `Entity` type has no etag member. So
the server's ETag (`api_v1.go:767-768` on GET, `write_handler.go:454-455` on
PATCH) is dropped at the client boundary.
   - Callers verified as passing NO etag: autosave property/content/relations
channels (`useAutoSave.ts:318-320, 369-371, 415-417`, literal `undefined`),
**explicit save** (`DynamicForm.vue:887`), Kanban drag-drop
(`KanbanView.vue:277`), list bulk-set (`useListActions.ts:76`). Only
`relaBridge.ts:95` forwards one, and only if a hosted app supplies it.
   - Optimistic locking is therefore **fully implemented server-side and fully
unused client-side**.

2. **The documented conflict strategy does not exist.** `useAutoSave.ts:18-20`
says *"cross-tab conflicts resolve through the SSE merge path."* Verified: SSE
`entity:changed` carries `{"type": "..."}` ONLY — no entity id, no payload
(`watcher.go:448-450`), deliberately, to avoid a per-entity existence oracle
(TKT-POT9GQ, `useEvents.ts:8-20`). No edit-form component subscribes:
`EntityView.vue` / `EntityDetail.vue` / `DynamicForm.vue` have no
`entity:changed` handler. The only two registered handlers are document
re-renderers (`DocumentView.vue:129`, `DocumentsPanel.vue:137`).
`mergeServerResponse` is called ONLY from useAutoSave's own PATCH-response
handlers (`:321, :372, :418`) — it never sees a server-pushed change. So
**cross-TAB edits are silently lost too**, not just cross-user.

3. **`dirtyFormRegistry` is dead weight.** Built specifically so *"an
SSE-triggered re-fetch doesn't clobber a user's in-progress keystrokes"*
(`dirtyFormRegistry.ts:1-9`), populated by `DynamicForm.vue:1247` — but
`anyFormDirty` has ZERO production consumers.

4. **The ETag is whole-entity, which drives the design.** `computeEntityETag`
(`api_v1.go:1805-1837`) hashes id + type + content + ALL properties + ALL
**outgoing** edges (`Type|To` only), truncated to 8 bytes. NOT covered:
affordances, INCOMING relations, relation edge properties. Consequence: with
autosave's per-property PATCHes, any concurrent change to any field 412s a save
touching a completely unrelated field — so naive `If-Match` produces heavy
spurious 412s. **The merge+retry loop is what makes If-Match viable at all, not
a nicety.**

### Failure mode today

`mergeServerResponse` (`useAutoSave.ts:488`) applies server values per-property
but SKIPS locally-dirty fields (`if (k in pending) continue`). Alice and Bob
both edit the same field → Bob's dirty field ignores Alice's value → Bob's next
debounced PATCH overwrites her. No 412 (no header sent), no conflict UI, and —
because the version sweep debounces on idle and Bob's write reset the timer —
often no version row capturing Alice's text either. **Silent last-write-wins.**

Different-field concurrent edits merge correctly today and MUST keep doing so.

## Approach (agreed with user, 2026-07-25)

Client-side resolution, no API contract change. Making `If-Match` *mandatory*
server-side was rejected: it breaks every other client (MCP, CLI, Lua) and 412s
the disjoint-field case the merge already handles.

0. **Prerequisite (new, from investigation): retain the ETag client-side.**
Expose response headers through the axios wrapper (or a targeted variant) and
populate `EntityCache.etag` / an entity-adjacent field from GET *and* PATCH
responses. Without this nothing else in this ticket is possible. Scope carefully
— `client.ts` unwrapping to `.data` is used everywhere.
1. **Send `If-Match`** from autosave and explicit-save paths.
2. **On 412: refetch → three-way merge → re-PATCH** with the fresh ETag.
`lastSeenServer` / `lastSeenContent` are ALREADY the correct merge base —
written only from server responses, never client-sent values (the S5
design-review invariant, `useAutoSave.ts:164-168`). Per field:
   - base = `lastSeenServer[k]`, ours = attempted patch, theirs = refetched
   - `theirs === base` → only we changed it → keep ours
   - `ours === base` → only they changed it → take theirs
   - both differ, and differ from each other → genuine conflict
3. **Bounded auto-retry: 2-3 attempts, jittered backoff.** A popup offering only
"retry/cancel" asks the user to do what the software should. Surface UI only
after the bound is exhausted or a genuine conflict is detected.
4. **Markdown body: real diff3 text merge, like git.** Base is
`lastSeenContent`; content is line-oriented, so disjoint hunks auto-merge. **Use
an existing library** — correct diff3 is not small and the frontend has no
coverage enforcement to catch a subtle merge bug. No diff dependency exists
today; survey `node-diff3` / `diff` / `diff3` against bundle size (SPA bundle
~372KB, deliberately cached — `apps_handler.go:140-143`).
   - **NEVER write git-style conflict markers into the entity.** In git a
developer resolves markers in a working tree; here they would PERSIST to the
entity, land in version history, and render in the SPA. Overlapping hunks
surface as a conflict; they are not committed.

## Explicitly out of scope

- **CRDT / concurrent collaborative editing** (Google-Docs style). User was
explicit: simultaneous edit is not the common case, "some effort is probably
already good enough"; CRDT-style editing is a later possibility.
- Server-side API contract changes (mandatory `If-Match` for all clients).
- Reviving the SSE-into-open-form path (payload is type-only by security
design). Out of scope here, but see the note below.

## Alternative to weigh in design review

**Temporary edit lock** — the one alternative the user said they would consider.
Short-lived advisory lock when a form opens, so the second user is told up-front
rather than at save time. Document trade-offs: stale locks (crashed tab, closed
laptop) need a TTL and a steal path; it needs a server-side mechanism and a
lock-holder identity; it is a stronger UX guarantee (no lost work at all) but
much heavier than a client-side merge. Not mutually exclusive — a lock could be
added later for high-contention types.

## Open questions for planning / design review

- **Autosave debounce granularity vs. text merge.** Autosave fires mid-typing,
so "ours" is often a partial sentence — merging two in-flight partial bodies
differs from merging two finished paragraphs. Consider attempting the body merge
only on explicit save/commit, with autosave more conservative.
- **Whole-entity ETag + per-property PATCH** (finding 4): confirm the retry loop
makes the spurious-412 rate acceptable, and that a busy entity cannot livelock
within the retry bound.
- **Should `dirtyFormRegistry` be revived or deleted?** (finding 3) It is dead
code built for a path that was never wired. Out of scope to fix here, but it
should not be left as a misleading half-mechanism.
- **Correct the stale comment** at `useAutoSave.ts:18-20` regardless of outcome
— it documents a conflict strategy that does not exist.

## Relationship to other tickets

Complementary to **TKT-0IGI4V** (flush-on-author-change), not an alternative:
this ticket PREVENTS the silent overwrite; the flush makes the pre-overwrite
state recoverable in version history if one happens anyway.
