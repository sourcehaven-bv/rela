---
id: RR-TEZ92VI
type: review-response
title: FlowStepForm tests miss unchanged-value pruning, missing form, load error and 409
finding: FlowStepForm.test.ts has no case that unchanged values are not sent, a missing form, a load error, or the 409 stale path.
severity: minor
resolution: Added FlowStepForm tests for unchanged values not being sent, a default, a load error and the 409 stale path. The missing-form case was added earlier.
status: addressed
---
