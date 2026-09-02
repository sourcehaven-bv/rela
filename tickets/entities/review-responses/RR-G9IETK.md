---
id: RR-G9IETK
type: review-response
title: Fast path diverged from full build on multi-parent and error-policy shapes
finding: 'Two equivalence breaks: (1) multi_parent:first picked a DIFFERENT winning parent inside the subtree when the lexicographic winner lay outside it, so a drill showed a child and rolled span the full view attributed elsewhere. (2) multi_parent:error returned 200 on a drill when the second parent was outside the subtree while the full build 422s; the caveat''s ACL analogy was rejected as not load-bearing for a config-integrity check.'
severity: critical
resolution: 'Equivalence is now the enforced contract: the fast path DECLINES (full build answers) for multi_parent:error configs, for any edge from an out-of-subtree parent claiming an in-set node, and for cycles through the drilled root — all detected during edge linking. on_cycle:error (the DEFAULT) keeps the fast path with the cycle diagnostic scoped to the subtree, documented honestly on its own terms (a disjoint cycle is the root view''s diagnostic; declining would forfeit the fast path for every default config). Pinned by TestGantt_SubtreeDrillExternalParentMatchesFull (byte-equality on the previously-diverging shape) and TestGantt_SubtreeDrillPropertyEquivalence (8 seeded random graphs, EVERY node drilled and compared byte-for-byte against the full build''s subtree).'
status: addressed
---
