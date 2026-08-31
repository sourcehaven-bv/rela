---
id: RR-8E74XB
type: review-response
title: Depth-capped node was byte-identical to a leaf on the wire
finding: 'emitGanttNode had two ways to withhold children but only the budget path signalled it: a depth-capped node serialized with no children key (omitempty) and no flag — indistinguishable from a genuine leaf. The SPA papered over it by re-rooting on drill, so it would have bitten the next programmatic consumer. Not an oracle: the decision depends only on config MaxDepth and position in the already-gated tree.'
severity: minor
resolution: 'Added GanttNode.HasMoreChildren (has_more_children, omitempty), set on BOTH withholding paths (depth cap and budget cut), documented as distinct from the response-level Truncated. Pinned by the extended TestGantt_DepthCapFoldsBeyond (capped node sets it, fully-emitted parent does not). The SPA also uses it now: the drill fetch policy refetches with ?root= when the target carries has_more_children (fixing a latent client-side drill-into-empty), and the tooltip''s drill hint respects it.'
status: addressed
---
