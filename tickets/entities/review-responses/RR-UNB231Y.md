---
id: RR-UNB231Y
type: review-response
title: Log message and unchecked reader assertion
finding: The drop message in runOnce did not fit delete jobs, and the deletedGate assertion was only promised by a comment.
severity: nit
resolution: Reworded the message and added compile-time assertions for the three readers.
status: addressed
---
