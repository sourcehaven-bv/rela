---
id: RR-PB92G4
type: review-response
title: Creator branch is not limited to creator types
finding: 'WaitingQuery has no CreatorTypes. The creator branch therefore returns every waiting-state channel the caller created, including types whose waiting roles are named roles. That inflates the 20 000 ceiling, and on fs it makes visitTypes walk every type in q.States. Fix: add CreatorTypes []string, apply it to the creator branch only, and on fs walk only those types for that branch.'
severity: minor
resolution: 'WaitingQuery gains CreatorTypes; the creator branch filters type IN CreatorTypes, and fs walks only those types for it. Conformance row: a creator channel of a non-creator type is absent.'
status: addressed
---
