---
id: RR-4W5SH73
type: review-response
title: Handler cannot fail Startable alone
finding: fakeFlows used one error for all methods and returned a slice with the error.
severity: minor
resolution: fakeFlows has startErr; TestFlowsAPI_SubjectRunsStartable asserts a failed Startable fails the whole answer; the fake returns nil with an error.
status: addressed
---
