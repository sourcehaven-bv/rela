---
id: RR-HW0YDNC
type: review-response
title: No symptom-level test of a role_relations list on fsstore
finding: The fix is pinned at the store and in the ACL query but not by a list request on fsstore with a conflicted row.
severity: minor
reason: conferredfaces_test asserts the read-grant query sets SkipUnreadable and TestGraphQuery_SkipUnreadable runs it on fsstore. The dataentry test apps run on memstore so a symptom test needs new fsstore wiring.
status: deferred
---
