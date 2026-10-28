---
id: RR-B1E8QF
type: review-response
title: Test gaps around watch and wiring
finding: No test for store-event replay, merged wake/poll, retry, run-as plumbing, or unknown faces.
severity: minor
resolution: Added TestWatch_ListCommentsReplaysOnStoreEvent, TestWatchRun_CommentsDueSurvivesMergedPasses, TestLocalIdentity_RunAs, unknown-face cases; run-as now goes through newMCPServices. End to end over real stdio verified manually (IMPL-V4KU47).
status: addressed
---
