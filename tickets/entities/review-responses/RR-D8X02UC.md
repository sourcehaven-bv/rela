---
id: RR-D8X02UC
type: review-response
title: acl.Graph probes carry no edge properties
finding: HasEdge/OutgoingRelations/batch methods return ids only, so Go probes cannot apply the condition.
severity: significant
resolution: 'Plan: pass the bound condition into the Graph methods; guard test that every implementation honours it.'
status: addressed
---
