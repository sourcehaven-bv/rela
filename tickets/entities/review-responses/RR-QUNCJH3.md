---
id: RR-QUNCJH3
type: review-response
title: A bad log fails the page
finding: addFinishedDetail returned an error for a header log that does not decode, so one bad header failed Run and SubjectRuns.
severity: significant
resolution: An unreadable log is logged with slog.Warn and the run shows without a timeline; TestFlowControl_FinishedRunWithoutLog covers a missing and an unreadable body.
status: addressed
---
