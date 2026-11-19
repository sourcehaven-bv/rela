---
id: RR-HRQRFRG
type: review-response
title: No test for an actor whose title the reader may not see
finding: The actor test covered a login and a visible person only; a later change reading the actor through an ungated path would leak the title without failing a test.
severity: significant
resolution: 'TestFlowControl_TimelineNamesActor is now a table: a reader who may see the actor''s title; a reader whose visible: grant hides the display field (By stays and ByTitle is empty); and a policy without person mapping.'
status: addressed
---
