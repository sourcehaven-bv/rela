---
id: RR-4G6V0
type: review-response
title: workspace.updateRelation godoc doesn't document merge vs replace semantics
finding: |-
    Plan changes merge-only to merge-then-unset additively. But existing godoc says 'updates properties' without specifying merge. Future caller assumes 'set' semantics, gets bitten when partial Properties merges. Recommendation: rewrite godoc to specify merge then unset then conditional content replacement; same for entitymanager.RelationOptions.

    From design-review: F12.
severity: minor
resolution: Plan Layer 0 includes the rewritten godoc text for `entitymanager.RelationOptions` and `workspace.updateRelation` documenting merge-then-unset semantics for UpdateRelation, full-replacement implication for CreateRelation, and `*string Content` nil-vs-empty-string distinction.
status: addressed
---
