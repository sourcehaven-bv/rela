---
id: RR-YRD34H
type: review-response
title: Depth:64 was silently clamped to 5 — drilled subtree truncated and hid deep overruns
finding: 'Every backend clamps RelationPredicate.Depth to 5 (graphquerynaive.DepthCap; pgstore cappedDepth mirrors it), so the subtree closure with Depth:64 truncated at ~depth 6: on a 9-deep chain the drill dropped the deepest nodes, returned rolled=null for the root, and reported a five-year overrun as on-schedule — silently, with no truncated flag. The equivalence test''s 4-node fixture could not catch it.'
severity: critical
resolution: 'The closure is now resolved ITERATIVELY (ganttClosureRoundDepth=5 per round, matching the documented clamp; frontier fed back until the set stabilizes), with a rounds backstop that DECLINES to the full build rather than truncating — silently returning a smaller, differently-folded tree named as the one unacceptable failure mode in the code comment. Pinned by TestGantt_SubtreeDrillDeepChain (9-deep chain: complete node count, complete fold, breach survives) and the generated-graph property test.'
status: addressed
---
