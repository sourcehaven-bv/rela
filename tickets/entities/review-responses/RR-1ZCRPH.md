---
id: RR-1ZCRPH
type: review-response
title: Wildcard or faced ceiling field keys load inert
finding: 'A client_baselines visible:/redact: key of * or type@face loads but FieldCeilingFor looks up the bare type, so it hides nothing.'
severity: significant
resolution: 'Restriction.validateTypeOnlyEntries now rejects * and @ keys in visible:/redact: at load; pinned in TestRestriction_RefusesFaceShapedEntriesAndWorldWildcards.'
status: addressed
---
