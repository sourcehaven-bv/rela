---
id: RR-3EK9TV
type: review-response
title: Breadth ignored MatchingIDs, the ACL membership-walk batch
finding: 'Breadth recorded EntityQuery.IDs and RelationQuery.EntityIDs, and its class comment asserted these were ''the two BATCH fields ... the ones a batching read path grows''. There is a third: store.MatchingIDs(ctx, q, ids) is the ACL membership walk, documented across acl/request.go, search/visible.go and dataentry/sync.go as one round-trip per distinct type rather than per row - precisely a batched read whose width grows with the row set, on the read path this test exercises. An over-fetch widening the ACL batch would be invisible to the instrument by construction.'
severity: significant
resolution: 'Breadth now records MatchingIDs on both the decorator and the Tx view. The class comment corrected from ''the two'' to three carriers and names what each is. Also added an explicit statement of what Breadth does NOT measure: a type-filtered query with no id list reads a whole type and records 0, so breadth bounds the argument a caller chose to pass, not how many entities a read returns.'
status: addressed
---
