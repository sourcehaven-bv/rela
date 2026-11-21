---
id: RR-43AR8TS
type: review-response
title: "A 422 without field errors is still logged"
finding: "The log moved into the branch without field errors, so a refusal without fields still reached console.error."
severity: minor
resolution: "Addressed: The form logs only a failure without a response status or with a 5xx status. Pinned in DynamicForm.refusal.test.ts."
status: addressed
---
