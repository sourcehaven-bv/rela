---
id: RR-1BEIUGC
type: review-response
title: TestErrorDescriptions lacks the empty case
finding: No test for an Error with no detections.
severity: nit
reason: make([]string, 0) is trivially correct; a test adds nothing.
status: wont-fix
---
