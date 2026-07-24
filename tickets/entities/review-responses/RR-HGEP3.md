---
id: RR-HGEP3
type: review-response
title: Rename stitch-fold and AllLifetimes-over-stitched-lineages were untested
finding: 'relation_lifetime_test.go only exercised create/delete lifetimes — the two cleverest parts of the diff (the claimed-set fold that stitches a pre-#1127 forked rename into ONE lifetime, and AllLifetimes completeness over a stitched pair) were unproven, and no test showed a renamed-away lineage is excluded from the old key.'
severity: minor
resolution: 'Added TestListRelationLifetimes_RenamedAwayNotListedUnderOldKey (heads filter on final endpoints), TestListRelationLifetimes_ForkedRenameStitchesToOneLifetime (fork collapses to one lifetime; AllLifetimes-over-stitched purge is refused by the rename guardrail as expected). Also added a defensive id-dedup in resolvePurgeLineage AllLifetimes with a comment pinning the disjointness invariant to ListRelationLifetimes.'
status: addressed
---
