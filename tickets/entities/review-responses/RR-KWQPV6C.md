---
id: RR-KWQPV6C
type: review-response
title: 'Test gaps: Go-only current_user, no identity, denied traversal'
finding: No test covered a current_user filter that stays on the Go path, a current_user filter without an identity, or QueryConditions under a policy that denies the far type.
severity: minor
resolution: Added mine-from-b to the differential, TestHandleListEntities_NoIdentityFailsClosed and TestGatedReads_ConditionTraversalsGatedByReader.
status: addressed
---
