---
id: RR-PZC4E9E
type: review-response
title: Multi-edge PATCH of an unbounded ruled relation was not atomic
finding: 'Found while fixing the combined PATCH: an unbounded relation wrapper writes its edges one by one, so removing two of two plans removed the first and refused the second.'
severity: significant
resolution: A wrapper of a relation a rule reads is written through ReplaceRelations, as a bounded one is (entitymanager.ChecksRelation). Pinned by the removing-every-edge case.
status: addressed
---
