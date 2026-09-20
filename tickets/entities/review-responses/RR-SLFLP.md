---
id: RR-SLFLP
type: review-response
title: Duplicating a self-loop silently creates a mutual pair, not a self-loop
finding: 'GET /{plural}/{id}/relations returns a self-loop TWICE — once under the canonical key (direction: outgoing) and once under the inverse key (direction: incoming) — because the store''s direction filter is pure endpoint matching and the handler has no self-loop dedup. Verified against a running server: after creating TKT-001 blocks TKT-001, the read returns both `blocks: [(TKT-001, outgoing)]` and `blockedBy: [(TKT-001, incoming)]`. buildDuplicatePrefill passes both keys through, so a duplicate with both groups checked emits both. The create SUCCEEDS (detectSelfLoopShapeConflict is inert here: it keys on the path entity, which for a create is a fresh id no body can name), and writes TWO DIFFERENT edges — verified on disk as TKT-007--blocks--TKT-001 and TKT-001--blocks--TKT-007. So duplicating a self-blocking ticket yields a copy that MUTUALLY blocks the original, which is not what the source expressed. Wrong output, no error.'
severity: critical
resolution: Self-loops are dropped and reported. Reproducing one needs the copy's own id, which does not exist at prefill time; carrying either key links the copy to the SOURCE and carrying both makes them point at each other. Reported once though the edge arrives under two keys. Pinned by two tests in duplicatePrefill.test.ts, including that other peers of the same relation still carry.
status: addressed
---

## Suggested resolution

Dedup by (canonical relation, peer id) before emitting. A peer equal to the
source id is the same physical edge under both keys; carry it once, under the
canonical key, so the copy self-loops as the source did — or drop it with an
`omitted` notice. AC18 asked whether the payload could be built so the conflict
cannot arise; it can, but 'no error' turned out not to mean 'right answer'.
