---
id: RR-XQRP3
type: review-response
title: TestHandleUpdateEntity_DeleteAbsentPropertyIsNoOp is over-clever
finding: 'tools_test.go. Inline comment says ''simpler: delete status from REQ-002 then call again'' — this is a smell. Either add a non-required property to the test metamodel and test directly, or split into a focused fresh-entity test. Currently reads like a committed debugging session.'
severity: minor
resolution: Reworked TestHandleUpdateEntity_DeleteAbsentPropertyIsNoOp. Added a non-required `priority` property to the test metamodel; the test now deletes `priority` on REQ-001 (which never had it set) directly, with no preliminary 'first delete' setup. Removed the apologetic inline comment.
status: addressed
---
