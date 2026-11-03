---
id: RR-BL7B5N
type: review-response
title: E2E spec leaves created tasks behind
finding: afterEach deletes the bug but not the tasks the test created.
severity: nit
resolution: afterEach now deletes the tasks related to the bug before deleting the bug.
status: addressed
---
