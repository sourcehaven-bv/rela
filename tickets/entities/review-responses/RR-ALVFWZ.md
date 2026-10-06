---
id: RR-ALVFWZ
type: review-response
title: Renumber plans pair siblings by triple
finding: manager_order.go and cli/renumber.go keyed siblings by from--type--to, collapsing two tails of one triple into one edge.
severity: significant
resolution: Both plans key by Relation.Identity(). Pinned by TestMaybeRenumberSide_TwoTailsOfOneTriple and TestRenumber_FacedTails. The outgoing list spans every tail of the source; documented at runRenumberAfterUpdate.
status: addressed
---
