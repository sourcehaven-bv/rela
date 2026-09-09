---
id: BUG-J7PXAC
type: bug
title: FuzzCloneNestedValues reports correct property-key refusals as crashes
description: 'The storetest fuzz target FuzzCloneNestedValues asserted require.NoError on CreateEntity for whatever property name the fuzzer produced. A name carrying a NUL or invalid UTF-8 is refused by storeutil.ValidateProperties — correctly, and by every backend — so the weekly fuzz sweep (#993) reported three correct refusals as three crashes, on fsstore, memstore and sqlitestore alike. The stores were never wrong; the target was missing the directional validity oracle that FuzzPropertyValuesTypeZoo already applies and whose contract the ValidateProperties godoc states: anything the shared rule rejects, every store MUST reject. A second, latent defect sat beside it: Go''s % keeps the sign, so a negative valueType matched no case and the entity went in with no property at all, making every clone assertion below it vacuous.'
priority: low
effort: xs
why1: FuzzCloneNestedValues called require.NoError on CreateEntity unconditionally, so a store correctly refusing an invalid property key was recorded as a test failure.
why2: The target applied no validity oracle to the property name — it guarded only entity.IsReservedEntityKey, which says nothing about NUL or invalid UTF-8.
why3: The oracle pattern (assert the store refused for the shared rule's own reason, then stop) was added to FuzzPropertyValuesTypeZoo and to createEntityOrSkip for IDs, but was never propagated to the sibling clone target.
why4: Property-key validity was tightened in the shared layer (ValidateProperties, BUG-X7ICNM) without a sweep over every fuzz target that writes a fuzzer-chosen property name, so a target predating the rule kept asserting the pre-rule contract.
why5: 'Systemic: a fuzz target''s oracle is a duplicate model of a contract owned elsewhere. When the contract tightens, every hand-modeled copy goes stale and fails as a false positive — the same class TKT-PCLGGL recorded when hand-modeled ID checks went stale, and the reason those targets now delegate to the exported validator. The class recurred inside the fix itself: the first version copied TypeZoo''s ValidateProperty skip along with its oracle, without re-checking that the skip''s premise (a PropertyValues divergence, BUG-CQYD5X) existed on this code path — it does not, and the skip silently dropped live inputs (RR-0DLHGU). Borrowing a guard is the same failure as borrowing a rule: both need re-deriving at the new site.'
prevention: The target now delegates to storeutil.ValidateProperties instead of assuming success, matching FuzzPropertyValuesTypeZoo and createEntityOrSkip — so a future tightening of the shared rule propagates to this target for free rather than surfacing as a false crash. A stricter backend's refusal is tolerated rather than skipped up front, so names the shared rule accepts still reach the clone assertions (RR-0DLHGU). Repros are committed as per-backend regression seeds (NUL key on fsstore, sqlitestore and pgstore, invalid-UTF-8 key on memstore), and the weekly fuzz sweep keeps generating hostile property names against the gate.
status: done
---

## Found by

The weekly fuzz sweep (#993), run
[34091381930](https://github.com/sourcehaven-bv/rela/actions/runs/34091381930)
and reproduced locally across the full target set.

## Symptom

`FuzzCloneNestedValues` fails on three backends with what looks like a store
defect but is a correct refusal:

```
store: property key "\x00": contains NUL          (fsstore, sqlitestore)
store: property key "\xbc": invalid UTF-8         (memstore)
```

## Root cause

The target writes a fuzzer-chosen property name and asserts `require.NoError(t,
s.CreateEntity(bg, e))`. `storeutil.ValidateProperties` rejects a key containing
NUL or invalid UTF-8, and every backend applies that rule, so the assertion
turns a correct rejection into a crash.

The godoc on `ValidateProperties` already states the intended contract — it is
"the validity oracle the storetest fuzz targets enforce directionally" — and
`FuzzPropertyValuesTypeZoo` implements it. This target simply never adopted it.

## Fix

Apply the directional oracle: a key the shared rule rejects must be refused by
the store *for that reason*, after which the case stops; anything accepted
proceeds to the clone assertions unchanged.

A stricter backend may still refuse a name the shared rule accepts, since
`ValidateProperty`'s empty-and-`/` rule is enforced unevenly (BUG-CQYD5X). That
is tolerated with a bare early return rather than a skip up front, so an
accepted name reaches the clone assertions. Skipping those names outright — as
the first version of this fix did, copied from TypeZoo — drops live inputs,
because this target never calls `PropertyValues`, which is where that divergence
actually lives. See RR-0DLHGU.

Also fixes the sign bug in `valueType % 3` beside it, which let a negative input
store no property and pass every clone assertion vacuously.

## Not a store defect

No backend behaviour changes. The three stores agreed with each other and with
the shared rule throughout; only the harness disagreed.
