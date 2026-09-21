---
id: RR-2HRIHC
type: review-response
title: The state.KV to per-backend migration has no rollback story and can double-apply on downgrade
finding: 'The plan notes existing stores must migrate off state.KV but treats it as a one-way read-old/write-new. It does not address downgrade: a store upgraded to the new binary, then rolled back, presents an old binary with a stale state.KV marker while the authoritative record lives in the new location. Because the shape-edge walk that used to skip already-applied files is being removed, the old binary would re-run migrations that already ran, relying entirely on step idempotency. Needs an explicit decision on whether the old key is kept in sync during a transition window, and the docs must state that rollback across this change is not supported if that is the answer.'
severity: significant
resolution: Adopted the dual-write recommendation. For one release the new writer also writes the legacy state.KV marker (best-effort, never fatal), and the new reader prefers the new location, so a rollback lands on a marker that is current rather than absent or stale. The new state carries a format version so a future reader can distinguish absent from newer-than-me and refuse rather than silently re-baseline. Removal of the legacy write is a follow-up once no supported version reads it. Recorded in the plan's Approach and Risks.
status: addressed
---

## Finding

The plan lists "existing stores must migrate off `state.KV`" as a risk and
proposes a one-time read-old/write-new. It does not address the reverse
direction, and the reverse direction is where the data loss is.

## The downgrade path

1. Store is on the old binary: marker in `state.KV`, applied list recorded there.
2. Upgrade. The new binary reads the old marker, writes
`migrations/applied.json` (or the new table), and proceeds.
3. **Roll back** — a normal operational response to an unrelated problem.
4. The old binary reads `state.KV`. If step 2 deleted the old key, the old binary
sees an un-bootstrapped store and **silently re-baselines** (`gate.go:109-114`
adopts the live shape with no ceremony). Pending migrations become unreachable.
If step 2 left the old key, it is now stale — it does not know about anything
applied after the upgrade.

Either way the old binary can re-run already-applied migrations.

## Why this is worse than it used to be

Today `Resolve` skips a file whose `to` shape the store has already reached
(`resolve.go:53`), so a wrong applied-list is caught by a second, independent
check. **That backstop is being removed by this same ticket.** Afterwards, step
idempotency is the only thing standing between a stale applied-list and a
re-application.

Step idempotency is documented and real, but it is not uniform: a `lua:` step is
idempotent only if the operator wrote it that way, and `map_values` re-applied
after a subsequent edit can remap values that were already remapped. The plan
itself acknowledges idempotency is being promoted "from second line of defence
to *the* line of defence" — this is the scenario where that promotion is tested.

## Required decisions

1. **Is the old `state.KV` key deleted, or kept written during a transition
window?** Dual-writing for one release is the conventional answer and makes
rollback safe. Deleting immediately is simpler and makes rollback unsafe.
2. **If rollback is unsupported, say so in the release notes and the guide.** An
unsupported path that looks like it works is worse than one that refuses.
3. **Consider a format marker in the new state** so a future binary can tell "no
state yet" from "state written by a newer version" and refuse rather than
re-baseline. This is the same gap TKT-ZV8CH0 raises for the projection encoding,
and the same fix applies.

## Recommendation

Dual-write the old key for one release, and have the new reader prefer the new
location. Cheap, removes the whole class, and can be deleted on a later release
once no supported version reads it.

## Evidence

- `internal/datamigration/gate.go:109-114` — silent re-baseline on absent marker
- `internal/datamigration/resolve.go:53` — the backstop being removed
- `internal/datamigration/marker.go:51-69` — corrupt/absent both return (nil,nil)
- TKT-ZV8CH0 — the parallel "no format version" gap
