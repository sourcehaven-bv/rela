---
id: RR-XXT3YRR
type: review-response
title: GetEntity per end before the type is known
finding: Each end of each changed edge is read even when no type allowed at that end has a rule on the hop.
severity: minor
resolution: checkRelationRules now notes an end only when a type the relation allows there has a rule on that hop (anyReadsHop).
status: addressed
---
