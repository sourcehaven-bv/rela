---
id: RR-TN8IW9
type: review-response
title: TestRecovery_IndexAfterCrashMidUpdate name no longer matches what it tests
finding: 'internal/store/fsstore/recovery_test.go:427: the comments were rewritten to say "the file changed on disk while no store was open", but the name still claims a crash mid-update and a stale index. Rename it to match the body, for example TestRecovery_ExternalEditBetweenRunsIsRead.'
severity: nit
resolution: Renamed to TestRecovery_ExternalEditBetweenRunsIsRead.
status: addressed
---
