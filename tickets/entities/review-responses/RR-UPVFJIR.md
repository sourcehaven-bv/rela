---
id: RR-UPVFJIR
type: review-response
title: HolderLookup is the wrong tool for a status filter
finding: HolderLookup answers "the property names an identity" and exists for one realtime question.
severity: significant
resolution: A Headers method on visibility.HolderLookup composes the row gate with caller Props and Narrowing and reads headers in the store with plain equality. It re-tests the predicates on the redacted row. No new type.
status: addressed
---
