---
id: RR-20SHQW
type: review-response
title: A move steps over and densifies around hidden siblings
finding: PlaceOrder ran over every raw edge of the source. A step that changed nothing visible, or a midpoint value off from the visible neighbors, revealed that a hidden related entity existed and where (security and code review).
severity: significant
resolution: The handler passes the targets the principal can read as OrderPosition.Among; the manager plans among those only. Pinned by TestRelationPosition_StepIgnoresHiddenSiblings, TestUpdateRelation_PositionAmongVisible and TestUpdateRelation_PositionAmongDensifiesVisibleOnly.
status: addressed
---
