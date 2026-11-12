---
id: RR-KUA77YM
type: review-response
title: Test gaps in relation push
finding: No tests for a double 412 or a 409 on relation create or forgetConflicts on relation entries.
severity: minor
resolution: TKT-0FMYPEZ adds TestRelationPush_TwiceStaleIsCountedNotSent (double 412), TestRelationPush_ExistingRelationIsDeferred (409 on create) and TestRelationPush_RefusalIsForgottenWhenTheRelationGoes (forgetConflicts on relation entries) in rules_test.go.
reason: The paths share code with the entity push which has these tests; added the failed-merge and rule tests. Remaining cases go with step 10.
status: addressed
---
