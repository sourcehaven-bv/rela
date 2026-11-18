---
id: RR-FE0OREB
type: review-response
title: CopyRun left a half-copied run when its Update failed
finding: flowrun.CopyRun created the run in the target and then left it half-copied when the following Update failed.
severity: minor
resolution: CopyRun deletes the partial copy when the Update fails (TestCopyRun_RemovesPartialCopy). Commit f2f2dac83.
status: addressed
---
