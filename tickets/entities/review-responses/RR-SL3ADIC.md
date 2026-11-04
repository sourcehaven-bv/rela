---
id: RR-SL3ADIC
type: review-response
title: Relation affordance verdicts for edges whose ends have no id yet
finding: gateCreateRelationAffordances is shaped around one candidate and peers with ids. Plan C does not say how local-local and existing-local edges get their verdicts.
severity: significant
resolution: 'batchHandler.relationSources: a batch entity as source is judged as its candidate; an existing source is read through the gated reader with the batch head as path entity (incoming direction). Same verdict path as the single create.'
status: addressed
---
