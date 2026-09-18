---
id: RR-8T3RKW
type: review-response
title: Shared the batch loop, tightened sameContent, renamed the probe, pinned the boundary
finding: Four smaller items. (1) rename_face would end up with a second copy of the batch-over-Tx loop; extract the shape, not just the move body. (2) sameContent ignores Type, and it decides whether to SKIP the create and delete the source anyway, so a false positive destroys a row uncopied. (3) txProbe overrides 2 of store.Store's 11 write methods but its doc claims it records 'each write'; the next person reusing it for a step calling UpdateEntity gets a green test proving nothing. (4) Nothing asserts the batch count, so the min() slice bound is the kind of off-by-one a refactor breaks silently.
severity: minor
resolution: (1) Hoisted the local `move` type to a package-level faceMove and extracted applyMoves; both steps now route through one loop, so there is one place to forget the transaction rather than two. (2) sameContent compares Type, with the reason in its doc. (3) Renamed to faceMoveProbe with a doc saying what it does and does not observe, and noting the writes count is asserted alongside bareTxns so an unobserved write cannot hide behind an expected total. (4) TestMigrateFace_BatchesAtTheDocumentedBoundary asserts the transaction count at 0, 1, exactly updateBatchSize, +1 and x2.
status: addressed
---

Item 1 was already the right instinct and I had half-done it — `applyFaceMove`
existed but each step still owned its own loop. Doing the full extraction while
fixing `rename_face` meant the edge-loss fix (RR-GQ8Y83) landed in one place
instead of two.

Also fixed a `govet` shadow introduced by reading `DeleteResult` where the old
code discarded it.
