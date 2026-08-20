---
id: RR-QOJTMJ
type: review-response
title: 'PR-A: pgstore silently ignored AllStates - fail-closed hole'
finding: The transitional guards covered writes and GetEntityState but EntityQuery{AllStates:true} sailed through returning default rows with no error — 'raw storage truth' answered with a filtered view, the exact lie states_transitional.go exists to prevent, and the first AllStates consumers (undeclared-pointer detection, observer backfill) are integrity checks where a quiet wrong answer is worse than an error.
severity: significant
resolution: rejectStateQuery added to states_transitional.go and called from ListEntities (error-yielding iterator), ListEntityHeaders, ListEntitiesPage, and CountEntities. Deleted with the rest of the transitional file in PR-B.
status: addressed
---
