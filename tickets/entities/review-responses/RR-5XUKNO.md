---
id: RR-5XUKNO
type: review-response
title: Faced source and relation tail chosen by rank
finding: source_face is optional so a faced source would resolve by world rank, which then drives the update check, the relation tail and source values.
severity: significant
resolution: 'Plan: source_face is required for a faced source type; tail rule: as=to uses the source face for content-scoped relations, as=from uses the recipe face; identity-scoped uses the zero tail via the scope helpers. Tests for faced source and faced target in both directions.'
status: addressed
---
