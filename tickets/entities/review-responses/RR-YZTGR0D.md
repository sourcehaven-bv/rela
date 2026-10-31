---
id: RR-YZTGR0D
type: review-response
title: No test for an error after a full page
finding: 'Code review: The error test only covers an error inside the first page.'
severity: minor
resolution: 'TestRedactingSeq_ErrorAfterAFullPage: 64 redacted rows, then the error, and the 5 buffered rows are dropped. The godoc says earlier pages were already yielded.'
status: addressed
---
