---
id: RR-VW2FAC
type: review-response
title: Both source gates consulted the face-blind row gate without pairing it with faceReadable
finding: |-
    PermitsReadMany is face-BLIND by design; every other gate in internal/dataentry pairs it with faceReadable (see visibleHeaderIDs, which documents exactly this pairing for TKT-O7R2A1). Neither the new frontier gate nor the new collection-load gate did so.

    Consequence: a principal granted only `policy@published` could traverse THROUGH a draft-only entity to reach its descendants — the same reachability leak BUG-9Z20WH is about, re-opened one coordinate down. The row gate returns true for the entity id; only the face check can refuse it.

    Verified: with a viewer granted read: [policy@published, note] and a draft-only POL-DRAFT seeded, PermitsReadMany(policy, [POL-DRAFT]) = map[POL-DRAFT:true] while faceReadable(policy, draft) = false.

    Initially masked by RR-VW1FAC — the faced row never resolved, so the id was dropped for absence before the face check would have run. That masking is why the two findings had to be fixed together.
severity: critical
status: addressed
resolution: |-
    Both gates now apply faceReadable alongside the row verdict. The frontier gate reads the face off the header it already fetched; the collection-load gate reads it off the loaded entity's Face field.

    Pinned by TestACLViewTraversal_FrontierAppliesFaceGate. The test deliberately runs under a world that SELECTS the draft face, because under the default world a faced entity has no row at all and the id would be dropped for absence — making the test pass with the face gate deleted and prove nothing. Two preconditions are asserted rather than assumed (the row gate still permits the entity; the draft row still resolves in that world), so the test names itself if it ever stops exercising the face check.

    Mutation-checked: deleting the faceReadable call makes it fail with "frontier expanded a face-denied node: [POL-DRAFT]".
---

## Context

Found by code review of the BUG-9Z20WH fix. The lint tripwire
(`TestViewTraversalIsSourceGated`) now requires `faceReadable(` at both gate
sites, so a future refactor that drops the face half names itself.
