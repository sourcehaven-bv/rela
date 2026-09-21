---
id: RR-AGY8P7
type: review-response
title: Two comments pointed at the wrong mechanism
finding: Code review N1 and N2. (N1) listIndexSpec's `ranked` guard was commented as the thing that preserves index-name stability for unranked specs, but the reviewer verified by probe that listIndexName produces byte-identical output for nil, [][]string{nil,nil} and [][]string{{},{}} - so stability comes from listIndexName skipping empty entries, and the guard is redundant defence. The comment pointed the next reader at the wrong mechanism. (N2) newEntitySorter copies before sorting; the reviewer noted the per-parent child bucket is freshly allocated so its copy is redundant, though the parent-level and flat-section calls genuinely need it.
severity: minor
resolution: 'N1: comment rewritten to say the guard is belt and braces and that the backend''s index name already ignores empty value lists, with the remaining reason for nilling stated (spec comparability, which the dedup key and equality assertions see). I had independently verified the same stability property with a throwaway test before the review landed. N2: kept the copy uniform rather than adding a may-I-mutate parameter, and said so in the comment along with the bound (one slice per parent, capped by nestedNodeBudget) so nobody optimises the parent case away by symmetry.'
status: addressed
---
