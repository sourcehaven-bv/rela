---
id: RR-AOQ0P
type: review-response
title: 'AC #17 test is a tautology — doesn''t actually count writes'
finding: |-
    TestV1Patch_NoOpSuppression_NoWrites at api_v1_relations_test.go:761-786 only asserts the relation is still there with the right value. Plan promised counting writes via a counter wrapper around the underlying FS. As written, the test passes whether or not no-op suppression exists — tx.WriteRelation is idempotent on disk and graph re-replaces with logically equivalent edge.

    Fix: wrap the FS like failOnNthRenameFS but as a counter; assert Rename count == 0 on no-op PATCH. Infrastructure already in test file; just add a second wrapper.
severity: significant
resolution: Added countingFS wrapper and TestV1Patch_NoOpSuppressionWritesZeroFiles which asserts Rename count == 0 after a PATCH with relations matching current state. Real test of no-op suppression instead of the previous tautology.
status: addressed
---
