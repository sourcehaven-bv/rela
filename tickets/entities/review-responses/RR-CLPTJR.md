---
id: RR-CLPTJR
type: review-response
title: Copies write owning edges without the owning check
finding: applyCopyEdges created relations without CheckOwningEdge, so a copy could give a child a second owner.
severity: minor
resolution: applyCopyEdges calls CheckOwningEdge before each create. TestCopy_OwningEdgeFollowsTheRules.
status: addressed
---
