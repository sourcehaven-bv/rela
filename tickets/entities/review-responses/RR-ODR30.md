---
id: RR-ODR30
type: review-response
title: Missing AC for graph invariant on commit-failure (Phase 1 mid-flight)
finding: |-
    Plan repeatedly emphasizes graph is untouched on validation failure (AC #12, #18). But no AC specifically for the commit-failure (Phase 1 rename failure mid-flight) case at the graph level. AC #13 covers 'rolls back' but doesn't specify what 'rolls back' means for the graph when on-disk is partially borked.

    Fix: add AC: 'On Phase 1 commit failure (Nth rename fails), the graph is not mutated at all (tx.applyGraphMutations is gated on full commit success). Handler returns 500 with clear error; on-disk state may be partially inconsistent but in-memory state is precisely the pre-PATCH state. Test: failure-injection at Phase 1 step 3 of 5; assert pre-PATCH graph state preserved.'
severity: nit
resolution: 'AC #20 explicitly covers Phase 1 commit failure: ''In-memory graph state is precisely the pre-PATCH state (gated by tx.applyGraphMutations on full commit success). Test: failure-injection at the filesystem layer.'' Graph invariant explicitly tested.'
status: addressed
---
