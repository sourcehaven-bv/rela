---
id: RR-WF37WJJ
type: review-response
title: A create cut off by the round timeout can be written twice
finding: When Outputs.Create fails on its context after the write landed, writeOutput gave the claim back, so a later round created the entity again.
severity: minor
resolution: writeOutput keeps the claim when ctx has ended. TestDue_CutOffCreateIsNotWrittenAgain.
status: addressed
---
