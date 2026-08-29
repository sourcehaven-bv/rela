---
id: TKT-52OFC9
type: ticket
title: Seed the autosave merge base in DynamicForm and delete the dead EntityCache.etag
kind: refactor
priority: medium
effort: xs
status: done
---

Two small, independent client fixes split out of TKT-2VDVHF's design review
(RR-VQQQ60 and the `EntityCache.etag` observation). Both are real defects on
their own terms and land ahead of — and independently of — the per-field
precondition work, which is why they are not folded into that ticket.

## (a) DynamicForm never seeded the autosave merge base

`useAutoSave` records its baseline (`lastSeenServer` / `lastSeenContent`) from
exactly two entry points: the `initialServerSnapshot` option
(`useAutoSave.ts:254-255`) or an explicit `recordServerSnapshot` call (`:244`).
`SectionEditForm.vue:95` passes the former and `EntityDetail.vue:402` calls the
latter — **`DynamicForm` did neither**. Its `loadEntity()` fetched the entity and
assigned it straight into `formData` / `content` / `relations` without ever
handing it to the composable, so the main entity edit form ran with
`lastSeenServer = {}` until the first PATCH response came back.

Consequences, present today and independent of any conflict work:

- **No-op suppression could not fire.** `fireDue` drops an entry whose value
  still `deepEqual`s `lastSeenServer[key]` (`useAutoSave.ts:352-356`). With an
  empty baseline every key looks changed, so retyping a value identical to the
  stored one issued a pointless PATCH — a write, a version-sweep candidate, and
  an SSE broadcast for a change that did not happen.
- **It removes the foundation any future three-way merge needs.** A merge with
  no base degenerates: `base=undefined` for every key, and a diff3 over an empty
  base reports "both sides added the whole document" — a whole-file conflict on
  the very first attempt.

### Fix

`loadEntity` takes the snapshot at the point it already marks as fresh server
state (right after `hiddenPolicy.releaseAll()` / `formGeneration.value++`) and
**before** the spreads hand the entity to the form.

Two properties are load-bearing:

- **Taken before mutation, and cloned.** If the baseline aliased the object
  spread into `formData`, later keystrokes would mutate the "server" side too:
  base would always equal ours and a three-way merge would silently degenerate
  into a two-way one. The snapshot deep-copies `properties` / `relations` and is
  `Object.freeze`d, so an accidental write to the baseline fails loudly instead
  of corrupting it.
- **Ordering.** `loadEntity()` runs in `onMounted` *before* `useAutoSave` is
  constructed, so the first snapshot is parked in `_pendingServerSnapshot` and
  passed as `initialServerSnapshot`; later reloads (`loadEntity(true)` from the
  attachment and transition paths) go through `recordServerSnapshot` on the live
  instance.

## (b) `EntityCache.etag` was declared and never written

`stores/entities.ts` declared `etag?: string` on `EntityCache`, but **no path
ever wrote it** — `fetchEntity`, `create` and `update` all construct
`{entity, timestamp}`. Nothing read it either.

Deleted rather than populated. A declared-but-never-written ETag is a trap: it
invites the assumption that cached reads carry an ETag, so an `If-Match` built
from this cache would send `undefined` (silently degrading to last-write-wins)
or a stale value. That assumption is plausibly how the original TKT-2VDVHF plan
came to treat ETag retention as nearly-free. The replacement comment states the
real invariant: **a cached entity has no meaningful ETag, so the etag-bearing
read path must go to the network** — which is also RR-PP9UEF's near-term fix.

## Verification

`DynamicForm.test.ts` gains three cases asserting (a) through its only
externally visible consequence, no-op suppression (the baseline is closed-over
state with no accessor):

- retyping the loaded server value issues **no** PATCH;
- a genuinely different value still PATCHes;
- edit-then-revert issues no PATCH — the observable form of "the base did not
  alias the form copy", since it only suppresses if the baseline held still
  while the form moved.

Mutation-verified: removing the `recordServerBaseline` call fails 2 of the 3.
Full frontend suite green (123 files / 2008 tests), `vue-tsc` clean, lint 0
errors.
