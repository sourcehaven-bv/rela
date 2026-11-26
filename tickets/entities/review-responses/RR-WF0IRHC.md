---
id: RR-WF0IRHC
type: review-response
title: Missing tests for conflict, rollback and nested paths
finding: No test covers a revision conflict, a rollback after the hook ran, a step write with a second run on the subject, nested-Tx fail-closed roles, the depth bound on update, or a kept change taken through the job path.
severity: minor
reason: Added the type-mismatch, branch-cap and kept-change guard tests. The others need fault injection in the store Tx; the flowrun conformance suite and the create-start tests cover the shared paths. Follow-up is TKT-FHH2JNJ.
status: deferred
---
