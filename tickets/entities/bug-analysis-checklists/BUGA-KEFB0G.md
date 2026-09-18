---
id: BUGA-KEFB0G
type: bug-analysis-checklist
title: 'Analysis: Face migration is not atomic and rename does not validate face sets'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Reproduction

- [x] Both defects confirmed by reading, then pinned by tests written to fail
first. This bug was filed unverified from a review of an adjacent change, so
confirming it was step one.
- [x] **Rename**: a failing test (`TestRenameEntityType_RefusesDivergentFaceSets`)
proved `Validate` accepts a rename from a type declaring `draft`/`published`
into one declaring `live`. Both shapes were in hand and `ShapeProjection`
already carries `Faces`; the check simply never looked.
- [x] **Atomicity**: a store wrapper recording whether each write arrived through
a transaction view showed all 6 writes (3 moves × create+delete) reaching the
outer store directly.
- [x] Severity qualified rather than assumed. The step is idempotent
(`!e.Face.IsDefault()` skips moved rows), so a re-run does recover — the
practical impact is smaller than the shape suggests, which is what the ticket
asked to check.

## Root cause

- [x] Root cause identified for both, and they are the same omission: a step
that changes where a row **lives** was reasoned about as a change to what a row
**contains**. A content change is one write and needs no cross-type check; a
coordinate change is neither.
- [x] Why the code was written that way: the engine's batched `store.Tx` helper
(`forEachEntity`) wraps `UpdateEntity`, and a face move is a create plus a
delete, so the helper did not fit and the loop was written plainly. A
create+delete pair is a two-write operation that only *looks* like one.
- [x] 5-whys completed to a systemic cause (recorded on the bug).

## Scope

- [x] Blast radius: operator-shell trust boundary, no ACL involvement. The
rename fix is load-time only and strictly narrowing — it refuses migrations that
would strand rows, and accepts everything it accepted before.
- [x] Backend differences established from the `store.Store.Tx` contract rather
than assumed: pg and sqlite give rollback, fs/mem give serialization only. So
the atomicity fix is **partial on fs/mem**, where idempotent re-run remains the
documented recovery. Stated in the code comment rather than glossed.
- [x] Checked that the fix does not violate the "no slow I/O inside Tx" rule —
the moves are local store writes with no external calls.

## Fix plan

- [x] **Atomicity**: batch the moves through `store.Store.Tx`, reusing
`updateBatchSize` for the reason its own doc gives — one graph-wide transaction
stalls every other writer on pg. The per-move body is extracted to
`applyFaceMove` taking the transaction view, so the "never write through the
outer store inside Tx" rule is visible in the signature.
- [x] **Rename**: compare face sets in `Validate` via a `facesNotIn` helper and
refuse with the orphaned faces named, so the operator knows which to
`rename_face`/`migrate_face` first.
- [x] Both mutation-verified (see the implementation checklist).
